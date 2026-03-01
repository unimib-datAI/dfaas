// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or later license. See LICENSE and
// AUTHORS file for more information.

// Package openwhisk implements faasprovider.FaaSProvider for Apache OpenWhisk.
package openwhisk

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/constants"
)

// owAnnotation represents a single OpenWhisk action annotation.
// Value is json.RawMessage because the OpenWhisk API uses arbitrary JSON
// for annotation values (objects, booleans, numbers, or strings).
type owAnnotation struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// owAction is the relevant subset of an OpenWhisk action list response entry.
type owAction struct {
	Name        string         `json:"name"`
	Namespace   string         `json:"namespace"`
	Annotations []owAnnotation `json:"annotations"`
}

// annotation returns the string representation of the annotation value for key.
// If the raw value is a JSON string, it is unquoted. Otherwise the raw JSON
// bytes are returned as-is. Returns "", false if the key is not found.
func (a owAction) annotation(key string) (string, bool) {
	for _, ann := range a.Annotations {
		if ann.Key == key {
			// If it's a JSON string, unquote it.
			var s string
			if err := json.Unmarshal(ann.Value, &s); err == nil {
				return s, true
			}
			// Otherwise return raw JSON representation.
			return string(ann.Value), true
		}
	}
	return "", false
}

// Client implements faasprovider.FaaSProvider for Apache OpenWhisk.
type Client struct {
	// host is "hostname:port" of the OpenWhisk API gateway (no scheme).
	host           string
	namespace      string
	apiKey         string
	prometheusHost string
	httpClient     *http.Client
}

// New returns a new OpenWhisk FaaSProvider using the default Prometheus origin.
// host must be in "hostname:port" form (no http:// prefix).
// namespace defaults to "guest" if empty.
// apiKey is the OpenWhisk API key ("uuid:key"); may be empty for open deployments.
func New(host, namespace, apiKey string) *Client {
	return NewWithPrometheus(host, namespace, apiKey, constants.PrometheusOrigin)
}

// NewWithPrometheus returns an OpenWhisk FaaSProvider with an explicit Prometheus host.
// prometheusHost must be in "hostname:port" form (e.g. "prometheus-server:80").
// namespace defaults to "guest" if empty.
func NewWithPrometheus(host, namespace, apiKey, prometheusHost string) *Client {
	if namespace == "" {
		namespace = "guest"
	}
	return &Client{
		host:           host,
		namespace:      namespace,
		apiKey:         apiKey,
		prometheusHost: prometheusHost,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

// doActionsRequest calls the OpenWhisk actions list endpoint and returns parsed actions.
func (c *Client) doActionsRequest() ([]owAction, error) {
	url := fmt.Sprintf("http://%s/api/v1/namespaces/%s/actions", c.host, c.namespace)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building OpenWhisk actions request: %w", err)
	}
	if c.apiKey != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(c.apiKey))
		req.Header.Set("Authorization", "Basic "+encoded)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET OpenWhisk actions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET OpenWhisk actions returned %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading OpenWhisk actions response: %w", err)
	}

	var actions []owAction
	if err := json.Unmarshal(body, &actions); err != nil {
		return nil, fmt.Errorf("parsing OpenWhisk actions response: %w", err)
	}
	return actions, nil
}

// GetFuncsWithMaxRates returns function names mapped to their dfaas.maxrate annotation value.
func (c *Client) GetFuncsWithMaxRates() (map[string]uint, error) {
	actions, err := c.doActionsRequest()
	if err != nil {
		return nil, err
	}
	result := map[string]uint{}
	for _, a := range actions {
		val, ok := a.annotation("dfaas.maxrate")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing dfaas.maxrate for action %q: %w", a.Name, err)
		}
		result[a.Name] = uint(n)
	}
	return result, nil
}

// GetFuncsNames returns the list of deployed action names.
func (c *Client) GetFuncsNames() ([]string, error) {
	actions, err := c.doActionsRequest()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, a := range actions {
		names = append(names, a.Name)
	}
	return names, nil
}

// GetFuncsWithTimeout returns function names mapped to their dfaas.timeout_ms annotation
// value in milliseconds, or nil if the annotation is absent.
func (c *Client) GetFuncsWithTimeout() (map[string]*uint, error) {
	actions, err := c.doActionsRequest()
	if err != nil {
		return nil, err
	}
	result := map[string]*uint{}
	for _, a := range actions {
		val, ok := a.annotation("dfaas.timeout_ms")
		if !ok {
			result[a.Name] = nil
			continue
		}
		n, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing dfaas.timeout_ms for action %q: %w", a.Name, err)
		}
		v := uint(n)
		result[a.Name] = &v
	}
	return result, nil
}

// HealthCheck returns the HTTP status (e.g. "200 OK") if the OpenWhisk controller
// is reachable. The Authorization header is sent when an apiKey is configured.
func (c *Client) HealthCheck() (string, error) {
	url := fmt.Sprintf("http://%s/api/v1/namespaces", c.host)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("openwhisk health check: creating request: %w", err)
	}
	if c.apiKey != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(c.apiKey))
		req.Header.Set("Authorization", "Basic "+encoded)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openwhisk health check: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	return resp.Status, nil
}

// promQuery runs a PromQL instant-query against the configured Prometheus host.
func (c *Client) promQuery(query string) ([]byte, error) {
	rawURL := fmt.Sprintf("http://%s/api/v1/query", c.prometheusHost)
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("openwhisk promQuery: building request: %w", err)
	}
	q := req.URL.Query()
	q.Add("query", query)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openwhisk promQuery %q: %w", query, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		return nil, fmt.Errorf("openwhisk promQuery %q: unexpected status %s", query, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// QueryAFET returns, for each OpenWhisk action, the Average Function Execution
// Time (in seconds) measured over the given time span.
// It uses openwhisk_action_duration_seconds_sum/count from the OpenWhisk exporter.
func (c *Client) QueryAFET(timeSpan time.Duration) (map[string]float64, error) {
	t := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	query := fmt.Sprintf(
		`rate(openwhisk_action_duration_seconds_sum[%s]) / rate(openwhisk_action_duration_seconds_count[%s])`,
		t, t,
	)
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var r owAFETResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("openwhisk QueryAFET: parsing response: %w", err)
	}
	result := map[string]float64{}
	for _, item := range r.Data.Result {
		if len(item.Value) < 2 {
			continue
		}
		val, _ := strconv.ParseFloat(item.Value[1].(string), 64)
		result[item.Metric.Action] = val
	}
	return result, nil
}

// QueryInvoc returns, for each OpenWhisk action, the invocation rate keyed by
// an HTTP-like status code string ("200" for success, "500" otherwise).
// It uses openwhisk_action_activations_total from the OpenWhisk exporter.
func (c *Client) QueryInvoc(timeSpan time.Duration) (map[string]map[string]float64, error) {
	t := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	query := fmt.Sprintf(`rate(openwhisk_action_activations_total[%s])`, t)
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var r owInvocResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("openwhisk QueryInvoc: parsing response: %w", err)
	}
	result := map[string]map[string]float64{}
	for _, item := range r.Data.Result {
		if len(item.Value) < 2 {
			continue
		}
		action := item.Metric.Action
		if _, ok := result[action]; !ok {
			result[action] = map[string]float64{}
		}
		code := owStatusToCode(item.Metric.Status)
		val, _ := strconv.ParseFloat(item.Value[1].(string), 64)
		result[action][code] = val
	}
	return result, nil
}

// owStatusToCode maps an OpenWhisk activation status label to an HTTP-like
// code string that matches the convention used by the rest of DFaaS.
func owStatusToCode(status string) string {
	if status == "success" {
		return "200"
	}
	return "500"
}

// QueryServiceCount returns, for each OpenWhisk action, the number of
// currently running Kubernetes deployment replicas in the OpenWhisk namespace.
// It uses kube_deployment_status_replicas filtered by the client namespace.
func (c *Client) QueryServiceCount() (map[string]int, error) {
	query := fmt.Sprintf(`kube_deployment_status_replicas{namespace="%s"}`, c.namespace)
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var r owServiceCountResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("openwhisk QueryServiceCount: parsing response: %w", err)
	}
	result := map[string]int{}
	for _, item := range r.Data.Result {
		if len(item.Value) < 2 {
			continue
		}
		val, _ := strconv.Atoi(item.Value[1].(string))
		result[item.Metric.Deployment] = val
	}
	return result, nil
}

// QueryCPUusage returns the CPU usage percentage for each node as reported by
// Prometheus node-exporter. The map key is the node instance label.
func (c *Client) QueryCPUusage(timeSpan time.Duration) (map[string]float64, error) {
	t := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	rawQuery := `1 - (avg by (instance) (rate(node_cpu_seconds_total{service="prometheus-prometheus-node-exporter", mode="idle"}[%s])))`
	query := fmt.Sprintf(rawQuery, t)
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var r owNodeMetricResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("openwhisk QueryCPUusage: parsing response: %w", err)
	}
	result := map[string]float64{}
	for _, item := range r.Data.Result {
		if len(item.Value) < 2 {
			continue
		}
		instance := item.Metric["instance"]
		val, _ := strconv.ParseFloat(item.Value[1].(string), 64)
		result[instance] = val
	}
	return result, nil
}

// QueryRAMusage returns the RAM usage percentage for each node as reported by
// Prometheus node-exporter. The map key is the node instance label.
func (c *Client) QueryRAMusage(timeSpan time.Duration) (map[string]float64, error) {
	t := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	rawQuery := `( 1 - (( avg_over_time(node_memory_MemFree_bytes[%s]) + avg_over_time(node_memory_Cached_bytes[%s]) + avg_over_time(node_memory_Buffers_bytes[%s]) ) / avg_over_time(node_memory_MemTotal_bytes[%s]) ) )`
	query := fmt.Sprintf(rawQuery, t, t, t, t)
	query = strings.Join(strings.Fields(query), " ")
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var r owNodeMetricResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("openwhisk QueryRAMusage: parsing response: %w", err)
	}
	result := map[string]float64{}
	for _, item := range r.Data.Result {
		if len(item.Value) < 2 {
			continue
		}
		instance := item.Metric["instance"]
		val, _ := strconv.ParseFloat(item.Value[1].(string), 64)
		result[instance] = val
	}
	return result, nil
}

// QueryCPUusagePerFunction returns, for each OpenWhisk action container, the
// fraction of total CPU it is using. Uses cAdvisor + node-exporter metrics.
func (c *Client) QueryCPUusagePerFunction(timeSpan time.Duration, funcName []string) (map[string]float64, error) {
	if len(funcName) == 0 {
		return map[string]float64{}, nil
	}
	t := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	funcFilter := strings.Join(funcName, "|")
	rawQuery := `sum by (container) ( irate(container_cpu_usage_seconds_total{container=~"%s"}[%s]) ) / on() group_left() sum by (instance) ( irate(node_cpu_seconds_total{service="prometheus-prometheus-node-exporter"}[%s]) )`
	query := fmt.Sprintf(rawQuery, funcFilter, t, t)
	query = strings.Join(strings.Fields(query), " ")
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var r owPerFunctionMetricResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("openwhisk QueryCPUusagePerFunction: parsing response: %w", err)
	}
	result := map[string]float64{}
	for _, item := range r.Data.Result {
		if len(item.Value) < 2 {
			continue
		}
		val, _ := strconv.ParseFloat(item.Value[1].(string), 64)
		result[item.Metric.Container] = val
	}
	return result, nil
}

// QueryRAMusagePerFunction returns, for each OpenWhisk action container, the
// fraction of total RAM it is using. Uses cAdvisor + node-exporter metrics.
func (c *Client) QueryRAMusagePerFunction(timeSpan time.Duration, funcName []string) (map[string]float64, error) {
	if len(funcName) == 0 {
		return map[string]float64{}, nil
	}
	t := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	funcFilter := strings.Join(funcName, "|")
	rawQuery := `( sum ( avg_over_time(container_memory_usage_bytes{container=~"%s"}[%s]) ) by(container) ) / on() group_left() ( avg_over_time(node_memory_MemTotal_bytes[%s]) )`
	query := fmt.Sprintf(rawQuery, funcFilter, t, t)
	query = strings.Join(strings.Fields(query), " ")
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var r owPerFunctionMetricResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("openwhisk QueryRAMusagePerFunction: parsing response: %w", err)
	}
	result := map[string]float64{}
	for _, item := range r.Data.Result {
		if len(item.Value) < 2 {
			continue
		}
		val, _ := strconv.ParseFloat(item.Value[1].(string), 64)
		result[item.Metric.Container] = val
	}
	return result, nil
}
