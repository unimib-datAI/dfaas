// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or later license. See LICENSE and
// AUTHORS file for more information.

// Package offuncs allows to retrieve the function lists and details from the
// local OpenFaaS instance.
package offuncs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"

	"go.uber.org/zap"
)

/*
	Example of response from "/system/functions":

	[
		{
			"name": "nodeinfo",
			"image": "functions/nodeinfo-http:latest",
			"invocationCount": 0,
			"replicas": 1,
			"envProcess": "",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "nodeinfo",
				"com.openfaas.uid": "181161398",
				"dfaas.maxrate": "50",
				"function": "true"
			},
			"annotations": null
		},
		{
			"name": "figlet",
			"image": "functions/figlet:0.13.0",
			"invocationCount": 0,
			"replicas": 1,
			"envProcess": "figlet",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "figlet",
				"com.openfaas.uid": "433840237",
				"dfaas.maxrate": "50",
				"function": "true"
			},
			"annotations": null
		},
		{
			"name": "funca",
			"image": "funca:latest",
			"invocationCount": 7501,
			"replicas": 1,
			"envProcess": "./handler",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "funca",
				"com.openfaas.uid": "107584837",
				"dfaas.maxrate": "110",
				"function": "true"
			},
			"annotations": null
		},
		{
			"name": "funcc",
			"image": "funcc:latest",
			"invocationCount": 2,
			"replicas": 1,
			"envProcess": "./handler",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "funcc",
				"com.openfaas.uid": "379141780",
				"dfaas.maxrate": "310",
				"function": "true"
			},
			"annotations": null
		},
		{
			"name": "funcb",
			"image": "funcb:latest",
			"invocationCount": 1,
			"replicas": 1,
			"envProcess": "./handler",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "funcb",
				"com.openfaas.uid": "764206021",
				"dfaas.maxrate": "210",
				"function": "true"
			},
			"annotations": null
		}
	]
*/

// funcsTimeoutResponse is a subset of the structure in a response from
// /system/functions. It contains only relevant attributes used by the
// "recalc" strategy.
type funcsMaxRatesResponse []struct {
	Name   string `json:"name"`
	Labels struct {
		MaxRate string `json:"dfaas.maxrate"`
	} `json:"labels"`
}

// funcsTimeoutResponse is a subset of the structure in a response from
// /system/functions. It contains only relevant attributes used by the
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

// funcsTimeoutResponse is a subset of the structure in a response from
// /system/functions. It contains only relevant attributes used by a generic
// strategy.
type funcsNamesResponse []struct {
	Name string `json:"name"`
}

// Client holds information for connecting to OpenFaaS instance.
type Client struct {
	hostname string
	port     uint
}

// NewClient returns a new Client.
func NewClient(hostname string, port uint) *Client {
	return &Client{hostname: hostname, port: port}
}

// doFuncsRequest gets info about functions from "/system/functions" endpoint.
func (c *Client) doFuncsRequest() ([]byte, error) {
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

/////////////////////////////////// PUBLIC INTERFACE ////////////////////////////////

// GetFuncsWithMaxRates returns the functions list as a map[string]uint of
// function names and dfaas.maxrate values.
func (c *Client) GetFuncsWithMaxRates() (map[string]uint, error) {
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

// GetFuncsNames returns the function names as list.
func (c *Client) GetFuncsNames() ([]string, error) {
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

// GetFuncsWithTimeout returns the functions list as a map[string]uint of
// function names and dfaas.timeout_ms values in milliseconds. If
// "dfaas.timeout" is not present, nil is set by default.
func (c *Client) GetFuncsWithTimeout() (map[string]*uint, error) {
	// Make the HTTP request to get function's metadata.
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	// Parse only relevant attributes.
	var respObj funcsTimeoutResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		logging.Logger().Debugf("Body response that fails JSON decoding: \n%s", string(body))
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	// Prepare output.
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
