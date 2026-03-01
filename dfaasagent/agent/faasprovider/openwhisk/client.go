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
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

// owAnnotation represents a single OpenWhisk action annotation.
type owAnnotation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// owAction is the relevant subset of an OpenWhisk action list response entry.
type owAction struct {
	Name        string         `json:"name"`
	Namespace   string         `json:"namespace"`
	Annotations []owAnnotation `json:"annotations"`
}

// annotation returns the value for the given annotation key, or ("", false) if absent.
func (a owAction) annotation(key string) (string, bool) {
	for _, ann := range a.Annotations {
		if ann.Key == key {
			return ann.Value, true
		}
	}
	return "", false
}

// Client implements faasprovider.FaaSProvider for Apache OpenWhisk.
type Client struct {
	// host is "hostname:port" of the OpenWhisk API gateway (no scheme).
	host      string
	namespace string
	apiKey    string
}

// New returns a new OpenWhisk FaaSProvider.
// host must be in "hostname:port" form (no http:// prefix).
// namespace defaults to "guest" if empty.
// apiKey is the OpenWhisk API key ("uuid:key"); may be empty for open deployments.
func New(host, namespace, apiKey string) *Client {
	if namespace == "" {
		namespace = "guest"
	}
	return &Client{host: host, namespace: namespace, apiKey: apiKey}
}

// doActionsRequest calls the OpenWhisk actions list endpoint and returns parsed actions.
func (c *Client) doActionsRequest() ([]owAction, error) {
	url := fmt.Sprintf("http://%s/api/v1/namespaces/%s/actions", c.host, c.namespace)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("building OpenWhisk actions request: %w", err)
	}
	if c.apiKey != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(c.apiKey))
		req.Header.Set("Authorization", "Basic "+encoded)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET OpenWhisk actions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET OpenWhisk actions returned %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
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

// HealthCheck returns "200 OK" if the OpenWhisk controller is reachable.
func (c *Client) HealthCheck() (string, error) {
	url := fmt.Sprintf("http://%s/api/v1/namespaces", c.host)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Status, nil
}

// QueryAFET returns "not yet implemented" — will be added in Task 7.
func (c *Client) QueryAFET(_ time.Duration) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryAFET not yet implemented for OpenWhisk")
}

// QueryInvoc returns "not yet implemented" — will be added in Task 7.
func (c *Client) QueryInvoc(_ time.Duration) (map[string]map[string]float64, error) {
	return nil, fmt.Errorf("QueryInvoc not yet implemented for OpenWhisk")
}

// QueryServiceCount returns "not yet implemented" — will be added in Task 7.
func (c *Client) QueryServiceCount() (map[string]int, error) {
	return nil, fmt.Errorf("QueryServiceCount not yet implemented for OpenWhisk")
}

// QueryCPUusage returns "not yet implemented" — will be added in Task 7.
func (c *Client) QueryCPUusage(_ time.Duration) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryCPUusage not yet implemented for OpenWhisk")
}

// QueryRAMusage returns "not yet implemented" — will be added in Task 7.
func (c *Client) QueryRAMusage(_ time.Duration) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryRAMusage not yet implemented for OpenWhisk")
}

// QueryCPUusagePerFunction returns "not yet implemented" — will be added in Task 7.
func (c *Client) QueryCPUusagePerFunction(_ time.Duration, _ []string) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryCPUusagePerFunction not yet implemented for OpenWhisk")
}

// QueryRAMusagePerFunction returns "not yet implemented" — will be added in Task 7.
func (c *Client) QueryRAMusagePerFunction(_ time.Duration, _ []string) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryRAMusagePerFunction not yet implemented for OpenWhisk")
}
