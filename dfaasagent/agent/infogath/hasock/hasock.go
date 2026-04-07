// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package hasock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

// STEntry represents a row of a stick-table
type STEntry struct {
	HTTPReqCnt  uint
	HTTPReqRate uint
}

type haproxyAPIEntry struct {
	Key         string `json:"key"`
	HTTPReqCnt  uint   `json:"http_req_cnt"`
	HTTPReqRate uint   `json:"http_req_rate"`
}

var _reStickTable *regexp.Regexp = nil

var dataplaneapi_url string
var dataplaneapi_username string
var dataplaneapi_password string

// Initialize sets up the hasock package with the custom Data Plane API host,
// port, username and password. Must be executed before any other calls of this
// package.
//
// FIXME: We should refactor this package with a struct that holds the
// connection info instead of using global shared variables!
func Initialize(host string, port uint, username, password string) {
	dataplaneapi_url = fmt.Sprintf("http://%s:%d", host, port)
	dataplaneapi_username = username
	dataplaneapi_password = password
}

// ReadStickTable reads the content of a stick table from the HAProxy Data Plane
// API. The stick table must be of type "http_req_cnt,http_req_rate(1s)"
func ReadStickTable(stName string) (map[string]*STEntry, error) {
	baseURL := fmt.Sprintf("%s/v3/services/haproxy/runtime", dataplaneapi_url)

	client := &http.Client{}

	// Get list of stick tables.
	listURL := fmt.Sprintf("%s/stick_tables", baseURL)
	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for stick table list: %w", err)
	}
	req.SetBasicAuth(dataplaneapi_username, dataplaneapi_password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling HAProxy stick_table list API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HAProxy list API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tables []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tables); err != nil {
		return nil, fmt.Errorf("decoding stick_table list response: %w", err)
	}

	// Check if stName is in the list.
	found := false
	for _, t := range tables {
		if t.Name == stName {
			found = true
			break
		}
	}
	if !found {
		return map[string]*STEntry{}, nil
	}

	// Fetch stick table entries.
	entriesURL := fmt.Sprintf("%s/stick_tables/%s/entries", baseURL, stName)
	req, err = http.NewRequest("GET", entriesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for stick table entries: %w", err)
	}
	req.SetBasicAuth(dataplaneapi_username, dataplaneapi_password)

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling HAProxy stick_table entries API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HAProxy entries API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var entries []haproxyAPIEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decoding entries JSON: %w", err)
	}

	// Convert to result format
	result := make(map[string]*STEntry)
	for _, entry := range entries {
		result[entry.Key] = &STEntry{
			HTTPReqCnt:  entry.HTTPReqCnt,
			HTTPReqRate: entry.HTTPReqRate,
		}
	}

	return result, nil
}
