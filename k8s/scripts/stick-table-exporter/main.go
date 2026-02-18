// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// stick-table-exporter is a small service that continuously polls (once per
// second) a stick-table value from HAProxy via the HAProxy Data Plane API and
// stores the observed values in a fixed-length buffer. It exposes an HTTP
// server with a single "/table" endpoint that returns the buffered samples,
// supporting both CSV and JSON output formats.
//
// Specifically, the exporter reads the gpt0 counter for the key "global" stored
// in the "main" stick-table. HAProxy uses this counter to track the most recent
// value of the "DFaaS-K6-Stage" request header. During load experiments, k6
// sets this header to indicate the current test stage, allowing synchronization
// of the active stage between the client (k6) and the server (the DFaaS node).
//
// Each sampled stage value is stored together with the corresponding UNIX
// timestamp (seconds resolution). The "/table" endpoint supports optional
// "start" and "end" query parameters (UNIX timestamps) to filter the returned
// data:
//
//   - If neither "start" nor "end" is provided, all buffered samples are returned.
//   - If only "start" is provided, results range from "start" to the current time.
//   - If only "end" is provided, results range from the oldest buffered sample
//     up to "end".
//   - If both are provided, only samples within [start, end] are returned.
//
// The program is configurable via environment variables. See the defined
// constants for the supported variables, their default values, and expected
// formats.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gammazero/deque"
)

const (
	// defaultRotationPeriod is the number of entries to keep in memory (by
	// default 1 per second if pollInterval is 1s).
	//
	// Configurable by ROTATION_PERIOD.
	defaultRotationPeriod = 12 * time.Hour

	// defaultHAProxyHost and defaultHAProxyPort specify the connection details
	// for the HAProxy Data Plane API.
	//
	// Configurable by HAPROXY_API_HOST and HAPROXY_API_PORT.
	defaultHAProxyHost = "localhost"
	defaultHAProxyPort = "30555"

	// defaultServerPort is the port to expose the HTTP server.
	defaultServerPort = "8080"

	// tableEndpoint is the HTTP endpoint to be exposed.
	tableEndpoint = "/table"

	// pollInterval defines how often to scrape the stick table.
	pollInterval = 1 * time.Second

	// adminUser and adminPassword are the credentials used to log in to the
	// HAProxy Data Plane API.
	adminUser     = "admin"
	adminPassword = "admin"
)

// StageData represents a single timestamped stage of "DFaaS-K6-Stage".
type StageData struct {
	// Timestamp is the UNIX timestamp in seconds.
	Timestamp int64 `json:"timestamp"`

	// Stage is the stage index of k6.
	Stage int `json:"stage"`
}

func main() {
	// Set retention via env var, or use the default value.
	retention := defaultRotationPeriod
	if v := os.Getenv("ROTATION_PERIOD"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Fatalf("Invalid ROTATION_PERIOD '%s': %v", v, err)
		}
		retention = d
	}

	// HAProxy Data Plane API configuration via env vars.
	apiHost := os.Getenv("HAPROXY_API_HOST")
	if apiHost == "" {
		apiHost = defaultHAProxyHost
	}
	apiPort := os.Getenv("HAPROXY_API_PORT")
	if apiPort == "" {
		apiPort = defaultHAProxyPort
	}

	// Build the URL for HAProxy Data Plane API.
	apiURL := fmt.Sprintf("http://%s:%s/v3/services/haproxy/runtime/stick_tables/main/entries", apiHost, apiPort)

	log.Printf("Configuration: ROTATION_PERIOD=%s, HAPROXY_API_HOST=%s, HAPROXY_API_PORT=%s",
		retention, apiHost, apiPort)

	// Compute maximum deque size: 1 entry per second.
	maxSize := int(retention.Seconds())
	store := new(deque.Deque[StageData])

	// The deque must be protected by a mutex since multiple goroutines will
	// read/write it!
	var storeMu sync.Mutex

	// Reduce connection setup overhead by reusing the same HTTP client.
	client := &http.Client{
		Timeout: 900 * time.Millisecond,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Create and run the ticker with the default interval of 1s.
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			// Run on a different goroutine to allow interleaved runs.
			go fetchStickTable(client, store, &storeMu, apiURL, maxSize)
		}
	}()

	// Expose HTTP endpoint /table.
	http.HandleFunc(tableEndpoint, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Incoming request: method=%s path=%s query=%s accept=%s",
			r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Accept"))

		// Start and end time must be UNIX timestamp.
		var startTime, endTime *int64
		if s := r.URL.Query().Get("start"); s != "" {
			t, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				http.Error(w, "Invalid start timestamp", http.StatusBadRequest)
				return
			}
			startTime = &t
		}
		if e := r.URL.Query().Get("end"); e != "" {
			t, err := strconv.ParseInt(e, 10, 64)
			if err != nil {
				http.Error(w, "Invalid end timestamp", http.StatusBadRequest)
				return
			}
			endTime = &t
		}

		storeMu.Lock()
		defer storeMu.Unlock()

		// If end is not provided, assume is Now().
		now := time.Now().Unix()
		if endTime == nil {
			endTime = &now
		}

		// If start is not provided, assume the timestamp is the oldest tracked
		// in the deque.
		if startTime == nil && store.Len() > 0 {
			oldest := store.Front().Timestamp
			startTime = &oldest
		}

		// Prevent nonsensical start/end time range.
		if *startTime > *endTime {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "Invalid query: start > end")
			return
		}

		// Accumulate here the entries to return.
		result := []StageData{}
		for i := 0; i < store.Len(); i++ {
			tick := store.At(i)

			// Apply start/end filter if provided.
			if (startTime == nil || tick.Timestamp >= *startTime) &&
				(endTime == nil || tick.Timestamp <= *endTime) {
				result = append(result, tick)
			}
		}

		// Return the entries as JSON or as CSV.
		accept := r.Header.Get("Accept")
		switch accept {
		case "text/csv":
			w.Header().Set("Content-Type", "text/csv")
			writer := csv.NewWriter(w)
			defer writer.Flush()
			writer.Write([]string{"timestamp", "stage"})
			for _, tick := range result {
				writer.Write([]string{strconv.FormatInt(tick.Timestamp, 10), strconv.Itoa(tick.Stage)})
			}
		default:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
		}
	})

	log.Println("Server running on port", defaultServerPort)
	log.Fatal(http.ListenAndServe(":"+defaultServerPort, nil))
}

// fetchStickTable reads the "General Purpose Table 0" (gpt0) field for the key
// "global" in the stick-table "main" from HAProxy. This field represents the
// current "Stage" of k6, which HAProxy stores by parsing the "DFaaS-K6-Stage"
// header from incoming requests.
func fetchStickTable(client *http.Client, store *deque.Deque[StageData], mu *sync.Mutex, apiURL string, maxSize int) {
	now := time.Now().Unix()
	stage := -1 // Default value if missing.

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Println("Failed to create HTTP request:", err)
		return
	}

	// Add basic auth (required to interact with HAProxy Data Plane API).
	req.SetBasicAuth(adminUser, adminPassword)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Failed to run HTTP request:", err)
		return
	}

	// Read and close immediately the body to allow to reuse the HTTP connection.
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Println("Failed to read HTTP response body:", err)
		return
	}

	// Parse result.
	switch resp.StatusCode {
	case 200:
		// First parse the JSON as an array of generic objects.
		var entries []map[string]interface{}
		if err := json.Unmarshal(body, &entries); err != nil {
			log.Println("Failed to parse JSON from HTTP response:", err)
			break
		}

		// Stick-table exists, but may be empty!
		if len(entries) > 0 {
			// We know there is only one entry (an object) with the key "gpt0",
			// we want this value as int (but it is float64 since it is JSON).
			if gpt0, ok := entries[0]["gpt0"].(float64); ok {
				stage = int(gpt0)
			} else {
				log.Printf("Found 'gpt0' is type %T, expected float64\n", entries[0]["gpt0"])
			}
		}
	case 503:
		log.Println("Stick table not available (HTTP 503)")
	default:
		log.Println("Unexpected HTTP status code:", resp.StatusCode)
	}

	mu.Lock()
	defer mu.Unlock()

	// Only check that the previous element is not the same timestamp (if exist).
	if store.Len() > 0 {
		lastEntry := store.Back()
		if lastEntry.Timestamp == now {
			log.Printf("Skipping duplicate timestamp: %d", now)
			return
		}
	}

	// Add current entry to the deque.
	store.PushBack(StageData{Timestamp: now, Stage: stage})

	// Remove old entries to maintain maxSize.
	for store.Len() > maxSize {
		store.PopFront()
	}

	// Get oldest and newest timestamps for logging.
	oldestTimestamp := int64(0)
	if store.Len() > 0 {
		oldestTimestamp = store.Front().Timestamp
	}

	log.Printf("HAProxy Data Plane API call completed: timestamp=%d stage=%d status=%d deque_size=%d oldest_timestamp=%d newest_timestamp=%d",
		now, stage, resp.StatusCode, store.Len(), oldestTimestamp, now)
}
