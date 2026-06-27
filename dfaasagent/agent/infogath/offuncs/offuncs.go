// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or later license. See LICENSE and
// AUTHORS file for more information.

package offuncs

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"

	"go.uber.org/zap"
)

// funcsMaxRatesResponse is a subset of the structure in a response from
// /system/functions using OpenFaaS
type funcsMaxRatesResponse []struct {
	Name   string `json:"name"`
	Labels struct {
		MaxRate string `json:"dfaas.maxrate"`
	} `json:"labels"`
}

// funcsTimeoutResponse is a subset of the structure in a response from
// /system/functions using OpenFaaS. It contains only relevant attributes used by the
// "alllocal" strategy.
type funcsTimeoutResponse []struct {
	Name   string `json:"name"`
	Labels struct {
		// Execution timeout for this function. This value is optional. When
		// provided, the HAProxy backend is configured accordingly, with a small
		// amount added to prevent premature rejection.
		Timeout *string `json:"dfaas.timeout_ms,omitempty"`
	} `json:"labels"`
}

// funcsNamesResponse is a subset of the structure in a response from
// /system/functions using OpenFaaS
type funcsNamesResponse []struct {
	Name string `json:"name"`
}

type FuncOpenWhisk struct {
	Name        string         `json:"name"`
	Annotations []OWAnnotation `json:"annotations"`
}

type OWAnnotation struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// Maxrate helper for OpenWhisk
func (f *FuncOpenWhisk) MaxRate() string {
	for _, ann := range f.Annotations {
		if ann.Key == "dfaas.maxrate" {
			return fmt.Sprintf("%v", ann.Value)
		}
	}
	return ""
}

// Timeout helper for OpenWhisk
func (f *FuncOpenWhisk) Timeout() *string {
	for _, ann := range f.Annotations {
		if ann.Key == "dfaas.timeout_ms" {
			val := fmt.Sprintf("%v", ann.Value)
			return &val
		}
	}
	return nil
}

// IsWebAction helper for OpenWhisk
func (f *FuncOpenWhisk) IsWebAction() bool {
	for _, ann := range f.Annotations {
		if ann.Key == "web-export" {
			switch v := ann.Value.(type) {
			case bool:
				return v
			case string:
				return v == "true"
			}
		}
	}
	return false
}

type Client interface {
	doFuncsRequest() ([]byte, error)
	GetFuncsWithMaxRates() (map[string]uint, error)
	GetFuncsNames() ([]string, error)
	GetFuncsWithTimeout() (map[string]*uint, error)
	GetWebActionPaths() (map[string]string, error)
}

type OpenFaaSClient struct {
	hostname string
	port     uint
}

type OpenWhiskClient struct {
	hostname  string
	port      uint
	auth      string
	namespace string
}

// NewOpenFaaSClient returns a new OpenFaaSClient
func NewOpenFaaSClient(hostname string, port uint) *OpenFaaSClient {
	return &OpenFaaSClient{
		hostname: hostname,
		port:     port,
	}
}

// NewOpenWhiskClient returns a new OpenWhiskClient
func NewOpenWhiskClient(hostname string, port uint, auth string, namespace string) *OpenWhiskClient {
	return &OpenWhiskClient{
		hostname:  hostname,
		port:      port,
		auth:      auth,
		namespace: namespace,
	}
}

// doFuncsRequest gets info about actions from "/api/v1/namespaces/" + c.namespace + "/actions" endpoint

func (c *OpenFaaSClient) doFuncsRequest() ([]byte, error) {
	u := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", c.hostname, c.port),
		Path:   "/system/functions",
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("HTTP GET to /system/functions: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET to /system/functions: %s", resp.Status)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP response from /system/functions: %w", err)
	}

	return body, nil
}

func (c *OpenWhiskClient) doFuncsRequest() ([]byte, error) {
	u := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", c.hostname, c.port),
		Path:   "/api/v1/namespaces/" + c.namespace + "/actions",
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	if c.auth != "" {
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(c.auth))
		req.Header.Set("Authorization", "Basic "+encodedAuth)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET to /api/v1/namespaces/%s/actions: %w", c.namespace, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET to /api/v1/namespaces/%s/actions: %s", c.namespace, resp.Status)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading HTTP response from /api/v1/namespaces/%s/actions: %w", c.namespace, err)
	}

	return body, nil
}

// GetFuncsWithMaxRates returns the functions list as a map[string]uint

func (c *OpenFaaSClient) GetFuncsWithMaxRates() (map[string]uint, error) {
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	var respObj funcsMaxRatesResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		logging.Logger().Debug("Body response that fails JSON decoding", zap.String("body", string(body)))
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	result := map[string]uint{}
	for _, item := range respObj {
		num, err := strconv.ParseUint(item.Labels.MaxRate, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing max rate integer: %v", err)

		}
		result[item.Name] = uint(num)
	}

	return result, nil
}

func (c *OpenWhiskClient) GetFuncsWithMaxRates() (map[string]uint, error) {
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	var respObj []FuncOpenWhisk
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		logging.Logger().Debug("Body response that fails JSON decoding", zap.String("body", string(body)))
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	result := make(map[string]uint)
	for _, item := range respObj {
		maxRate := item.MaxRate()
		if maxRate == "" {
			continue
		}
		num, err := strconv.ParseUint(maxRate, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing max rate integer for function %q: %v", item.Name, err)
		}
		result[item.Name] = uint(num)
	}

	return result, nil
}

// GetFuncsNames returns the function names as list

func (c *OpenFaaSClient) GetFuncsNames() ([]string, error) {
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	var respObj funcsNamesResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		logging.Logger().Debugf("Body response that fails JSON decoding: \n%s", string(body))
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	var result []string
	for _, item := range respObj {
		result = append(result, item.Name)
	}

	return result, nil
}

func (c *OpenWhiskClient) GetFuncsNames() ([]string, error) {
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	var respObj []FuncOpenWhisk
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		logging.Logger().Debug("Body response that fails JSON decoding", zap.String("body", string(body)))
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	var result []string
	for _, item := range respObj {
		result = append(result, item.Name)
	}

	return result, nil
}

// GetFuncsWithTimeout returns the functions list as a map[string]*uint

func (c *OpenFaaSClient) GetFuncsWithTimeout() (map[string]*uint, error) {
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	var respObj funcsTimeoutResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		logging.Logger().Debugf("Body response that fails JSON decoding: \n%s", string(body))
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	result := map[string]*uint{}
	for _, item := range respObj {
		if item.Labels.Timeout == nil {
			result[item.Name] = nil
			continue
		}

		num, err := strconv.ParseUint(*item.Labels.Timeout, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing timeout for function %q: %v", item.Name, err)
		}
		val := uint(num)
		result[item.Name] = &val
	}
	return result, nil
}

func (c *OpenWhiskClient) GetFuncsWithTimeout() (map[string]*uint, error) {
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	var respObj []FuncOpenWhisk
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		logging.Logger().Debug("Body response that fails JSON decoding", zap.String("body", string(body)))
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	result := make(map[string]*uint, len(respObj))
	for _, item := range respObj {
		timeoutStr := item.Timeout()
		if timeoutStr == nil {
			result[item.Name] = nil
			continue
		}
		num, err := strconv.ParseUint(*timeoutStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing timeout for function %q: %v", item.Name, err)
		}
		val := uint(num)
		result[item.Name] = &val
	}
	return result, nil
}

// needed for Client interface to compile
func (c *OpenFaaSClient) GetWebActionPaths() (map[string]string, error) {
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	var respObj funcsNamesResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		logging.Logger().Debugf("Body response that fails JSON decoding: \n%s", string(body))
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	result := make(map[string]string)
	for _, item := range respObj {
		path := fmt.Sprintf("/function/%s", item.Name)
		result[item.Name] = path
	}

	return result, nil
}

func (c *OpenWhiskClient) GetWebActionPaths() (map[string]string, error) {
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	var respObj []FuncOpenWhisk
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		logging.Logger().Debug("Body response that fails JSON decoding", zap.String("body", string(body)))
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	result := make(map[string]string)
	for _, item := range respObj {
		if item.IsWebAction() {
			path := fmt.Sprintf("/api/v1/web/%s/default/%s", c.namespace, item.Name)
			result[item.Name] = path
		}
	}

	return result, nil
}
