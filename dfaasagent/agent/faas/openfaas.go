// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or-later license. See LICENSE and
// AUTHORS file for more information.

package faas

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ofFunctionResponse contains the relevant attributes of a function
// returned by the /system/functions endpoint. Unspecified JSON fields are
// ignored.
type ofFunctionResponse struct {
	Name   string `json:"name"`
	Labels struct {
		MaxRate string  `json:"dfaas.maxrate"`
		Timeout *string `json:"dfaas.timeout_ms"`
	} `json:"labels"`
}

type ofFunctionsResponse []ofFunctionResponse

// OpenFaaS holds information for connecting to the local OpenFaaS instance.
type OpenFaaS struct {
	httpClient *http.Client
	address    string
}

// New returns a new OpenFaaS object.
func NewOpenFaaS(hostname string, port uint) (*OpenFaaS, error) {
	return &OpenFaaS{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		address:    net.JoinHostPort(hostname, strconv.FormatUint(uint64(port), 10)),
	}, nil
}

// functions retrieves function information from /system/functions.
func (o *OpenFaaS) functions() (ofFunctionsResponse, error) {
	u := url.URL{
		Scheme: "http",
		Host:   o.address,
		Path:   "/system/functions",
	}

	resp, err := o.httpClient.Get(u.String())
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

	var functions ofFunctionsResponse
	if err := json.Unmarshal(body, &functions); err != nil {
		return nil, fmt.Errorf("deserializing JSON functions info: %w", err)
	}

	return functions, nil
}

func (o *OpenFaaS) Platform() string {
	return PlatformOpenFaaS
}

func (o *OpenFaaS) Names() ([]string, error) {
	functions, err := o.functions()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(functions))
	for _, fn := range functions {
		names = append(names, fn.Name)
	}

	return names, nil
}

func (c *OpenFaaS) WithMaxRates() (map[string]uint, error) {
	functions, err := c.functions()
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

		maxRate, err := strconv.ParseUint(fn.Labels.MaxRate, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing MaxRate for function %q: %w", fn.Name, err)
		}
		result[fn.Name] = uint(maxRate)
	}

	return result, nil
}

func (o *OpenFaaS) WithTimeout() (map[string]*uint, error) {
	functions, err := o.functions()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*uint, len(functions))
	for _, fn := range functions {
		if fn.Labels.Timeout == nil {
			result[fn.Name] = nil
			continue
		}

		timeout, err := strconv.ParseUint(*fn.Labels.Timeout, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing Timeout for function %q: %w", fn.Name, err)
		}
		value := uint(timeout)
		result[fn.Name] = &value
	}

	return result, nil
}

func (o *OpenFaaS) HealthCheck() (string, error) {
	u := url.URL{
		Scheme: "http",
		Host:   o.address,
		Path:   "/healthz",
	}

	resp, err := o.httpClient.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("failed OpenFaaS healthcheck: %w", err)
	}
	defer func() {
		// Allow to reuse the same TCP connection for other requests.
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	return resp.Status, nil
}
