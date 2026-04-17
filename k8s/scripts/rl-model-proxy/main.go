package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Listen string
	Target string
	Map    string
}

func loadMapping(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m map[string]string
	err = json.Unmarshal(b, &m)
	return m, err
}

func main() {
	var cfg Config

	flag.StringVar(&cfg.Listen, "listen", ":8080", "listen address")
	flag.StringVar(&cfg.Target, "target", "", "upstream target (e.g. http://localhost:9000)")
	flag.StringVar(&cfg.Map, "map", "map.json", "JSON mapping file")
	flag.Parse()

	if cfg.Target == "" {
		log.Fatal("missing -target")
	}

	mapping, err := loadMapping(cfg.Map)
	if err != nil {
		log.Fatalf("failed to load map: %v", err)
	}
	log.Printf("Loaded mapping: %v", mapping)

	targetURL, err := url.Parse(cfg.Target)
	if err != nil {
		log.Fatalf("invalid target URL: %v", err)
	}

	// We build a slice with [src1, dst1, src2, dst2...]. This is needed to
	// build a Replacer.
	pairs := make([]string, 0, len(mapping)*2)
	for src, dst := range mapping {
		pairs = append(pairs, src, dst)
	}
	replacer := strings.NewReplacer(pairs...)

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		log.Printf("%s - %s %s", req.RemoteAddr, req.Method, req.URL.String())

		if req.Body == nil {
			return
		}

		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			log.Printf("read body error: %v", err)
			return
		}
		req.Body.Close()

		if len(bodyBytes) == 0 {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			return
		}

		newBody := replacer.Replace(string(bodyBytes))

		var buf bytes.Buffer
		buf.WriteString(newBody)

		req.Body = io.NopCloser(&buf)
		req.ContentLength = int64(buf.Len())
	}

	// Apply a similar tranformation also for the response, but with the mapping
	// reversed.
	reversePairs := make([]string, 0, len(mapping)*2)
	for src, dst := range mapping {
		reversePairs = append(reversePairs, dst, src)
	}
	reverseReplacer := strings.NewReplacer(reversePairs...)

	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.Body == nil {
			return nil
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()

		newBody, err := updateActionFormat(bodyBytes)
		if err != nil {
			return fmt.Errorf("failed to update action JSON format: %w", err)
		}

		newBodyStr := reverseReplacer.Replace(string(newBody))

		var buf bytes.Buffer
		buf.WriteString(newBodyStr)

		resp.Body = io.NopCloser(&buf)
		resp.ContentLength = int64(buf.Len())

		// Force Go to update the header with value from resp.ContentLength.
		resp.Header.Del("Content-Length")

		return nil
	}

	log.Printf("Proxy listening on %q with target %q", cfg.Listen, cfg.Target)
	log.Fatal(http.ListenAndServe(cfg.Listen, proxy))
}

// updateActionFormat returns a modified version of the JSON body from the
// action response.
//
// With the following fixed nodes: [node_0, ..., node_4]. Example input:
//
//	{"node_0":[0.9999946355819702,9.961642035705154e-07,1.2219852578709833e-06,1.0784980304379133e-06,1.2444977528502932e-06,9.146793331638037e-07]}
//
// Example output:
//
//  {"node_A":{"local":0.9999946355819702,"node_B":9.961642035705154e-7,"node_C":0.0000012219852578709833,"node_D":0.0000010784980304379133,"node_E":0.0000012444977528502932,"reject":9.146793331638037e-7}}
func updateActionFormat(body []byte) ([]byte, error) {
	var action map[string][]float64
	if err := json.Unmarshal(body, &action); err != nil {
		return nil, fmt.Errorf("unmarshalling original action JSON: %w", err)
	}

	// Make sure in the action JSON there is only one node!
	found := false
	var node string
	var values []float64
	for nodeKey, valuesOrig := range action {
		if found {
			return nil, fmt.Errorf("action JSON should contain only one node ID, found one more node called %q", nodeKey)
		}
		found = true
		node = nodeKey
		values = valuesOrig
	}

	updatedAction := make(map[string]map[string]float64)
	updatedAction[node] = make(map[string]float64)

	if len(values) != 6 {
		return nil, fmt.Errorf("expected action should have 6 sub-actions, found %d", len(values))
	}

	// These are always fixed indexes.
	updatedAction[node]["local"] = values[0]
	updatedAction[node]["reject"] = values[5]

	// Each action JSON array has a dynamic index for each node. Since in the
	// original action there is no reference to each node, we have a static list
	// for each node, extracted from the DFaaS environment.
    //
    // FIXME: This is an hack and should be removed!
	//
	// Se __init__() from DFaaS env in Python.
	graph := map[string][]string{
		"node_0": {"node_1", "node_2", "node_3", "node_4"},
		"node_1": {"node_0", "node_2", "node_3", "node_4"},
		"node_2": {"node_0", "node_1", "node_3", "node_4"},
		"node_3": {"node_0", "node_1", "node_2", "node_4"},
		"node_4": {"node_0", "node_1", "node_2", "node_3"},
	}
	for index, neighbor := range graph[node] {
		// The 0 index is local action (see above).
		updatedAction[node][neighbor] = values[index+1]
	}

	updatedActionBytes, err := json.Marshal(updatedAction)
	if err != nil {
		return nil, fmt.Errorf("marshalling updated action JSON: %w", err)
	}
	return updatedActionBytes, nil
}
