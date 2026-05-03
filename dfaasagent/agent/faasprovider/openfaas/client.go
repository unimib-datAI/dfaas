// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or later license. See LICENSE and
// AUTHORS file for more information.

// Package openfaas implements faasprovider.FaaSProvider for an OpenFaaS gateway.
package openfaas

import (
	"fmt"
	"net/http"
	"time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/ofpromq"
)

// Client implements faasprovider.FaaSProvider for an OpenFaaS gateway.
type Client struct {
	funcsClient *offuncs.Client
	hostname    string
	port        uint
}

// New returns a new OpenFaaS FaaSProvider.
func New(hostname string, port uint) *Client {
	return &Client{
		funcsClient: offuncs.NewClient(hostname, port),
		hostname:    hostname,
		port:        port,
	}
}

func (c *Client) GetFuncsWithMaxRates() (map[string]uint, error) {
	return c.funcsClient.GetFuncsWithMaxRates()
}

func (c *Client) GetFuncsNames() ([]string, error) {
	return c.funcsClient.GetFuncsNames()
}

func (c *Client) GetFuncsWithTimeout() (map[string]*uint, error) {
	return c.funcsClient.GetFuncsWithTimeout()
}

func (c *Client) QueryAFET(timeSpan time.Duration) (map[string]float64, error) {
	return ofpromq.QueryAFET(timeSpan)
}

func (c *Client) QueryInvoc(timeSpan time.Duration) (map[string]map[string]float64, error) {
	return ofpromq.QueryInvoc(timeSpan)
}

func (c *Client) QueryServiceCount() (map[string]int, error) {
	return ofpromq.QueryServiceCount()
}

func (c *Client) QueryCPUusage(timeSpan time.Duration) (map[string]float64, error) {
	return ofpromq.QueryCPUusage(timeSpan)
}

func (c *Client) QueryRAMusage(timeSpan time.Duration) (map[string]float64, error) {
	return ofpromq.QueryRAMusage(timeSpan)
}

func (c *Client) QueryCPUusagePerFunction(timeSpan time.Duration, funcNames []string) (map[string]float64, error) {
	return ofpromq.QueryCPUusagePerFunction(timeSpan, funcNames)
}

func (c *Client) QueryRAMusagePerFunction(timeSpan time.Duration, funcNames []string) (map[string]float64, error) {
	return ofpromq.QueryRAMusagePerFunction(timeSpan, funcNames)
}

func (c *Client) HealthCheck() (string, error) {
	strURL := fmt.Sprintf("http://%s:%d/healthz", c.hostname, c.port)
	resp, err := http.Get(strURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Status, nil
}
