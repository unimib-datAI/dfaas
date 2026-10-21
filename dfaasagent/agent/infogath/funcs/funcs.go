// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or-later license. See LICENSE and
// AUTHORS file for more information.

// Package funcs allows to retrieve the function lists and details from the
// local FaaS instance.
package funcs

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"go.uber.org/zap"
)

// functionResponse contains the relevant attributes of a function returned by
// the /system/functions endpoint. Unspecified JSON fields are ignored.
type functionResponse struct {
	Name   string `json:"name"`
	Labels struct {
		MaxRate string  `json:"dfaas.maxrate"`
		Timeout *string `json:"dfaas.timeout_ms"`
	} `json:"labels"`
}

type functionsResponse []functionResponse

// Client holds information for connecting to an OpenFaaS instance.
type Client struct {
	hostname   string
	port       uint
	httpClient *http.Client
}

// New returns a new Client.
func New(hostname string, port uint) *Client {
	return &Client{
		hostname: hostname,
		port:     port,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// doFuncsRequest retrieves function information from /system/functions.
func (c *Client) doFuncsRequest() ([]byte, error) {
	u := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(c.hostname, strconv.FormatUint(uint64(c.port), 10)),
		Path:   "/system/functions",
	}

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("HTTP GET to /system/functions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Drain the body when possible to allow connection reuse.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("HTTP GET to /system/functions: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP response from /system/functions: %w", err)
	}

	return body, nil
}

// getFunctions retrieves and decodes the function list.
func (c *Client) getFunctions() (functionsResponse, error) {
	body, err := c.doFuncsRequest()
	if err != nil {
		return nil, fmt.Errorf("get functions info: %w", err)
	}

	var functions functionsResponse
	if err := json.Unmarshal(body, &functions); err != nil {
		logging.Logger().Debug(
			"Body response that fails JSON decoding",
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	return functions, nil
}

// parseUintLabel parses an unsigned 32-bit integer from a function label.
func parseUintLabel(value, label, functionName string) (uint, error) {
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf(
			"parsing %s for function %q: %w",
			label, functionName, err,
		)
	}

	return uint(n), nil
}

// WithMaxRates returns a map of function names to dfaas.maxrate values.
func (c *Client) WithMaxRates() (map[string]uint, error) {
	functions, err := c.getFunctions()
	if err != nil {
		return nil, err
	}

	result := make(map[string]uint, len(functions))
	for _, fn := range functions {
		if fn.Labels.MaxRate == "" {
			return nil, fmt.Errorf(
				"max rate label not found for function %q",
				fn.Name,
			)
		}

		maxRate, err := parseUintLabel(
			fn.Labels.MaxRate, "max rate label", fn.Name,
		)
		if err != nil {
			return nil, err
		}

		result[fn.Name] = maxRate
	}

	return result, nil
}

// Names returns the names of all functions.
func (c *Client) Names() ([]string, error) {
	functions, err := c.getFunctions()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(functions))
	for _, fn := range functions {
		names = append(names, fn.Name)
	}

	return names, nil
}

// WithTimeout returns a map of function names to dfaas.timeout_ms
// values in milliseconds. A nil value indicates that the timeout is absent.
func (c *Client) WithTimeout() (map[string]*uint, error) {
	functions, err := c.getFunctions()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*uint, len(functions))
	for _, fn := range functions {
		if fn.Labels.Timeout == nil {
			result[fn.Name] = nil
			continue
		}

		timeout, err := parseUintLabel(
			*fn.Labels.Timeout, "timeout label", fn.Name,
		)
		if err != nil {
			return nil, err
		}

		value := timeout
		result[fn.Name] = &value
	}

	return result, nil
}
