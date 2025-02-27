// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package handles a web server to expose endpoints on the agent (e.g.
// endpoint for DFaaS node healthcheck)
package httpserver

import (
	"io"
	"net/http"
	"strconv"
	"fmt"

	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/config"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/forecaster"
)

//////////////////// MAIN PRIVATE VARS AND INIT FUNCTION ////////////////////

var _config config.Configuration
var _forecasterClient forecaster.Client

// Initialize initializes this package (sets some vars, etc...)
func Initialize(config config.Configuration) {
	_config = config

	_forecasterClient = forecaster.Client{
		Hostname: _config.ForecasterHost,
		Port:     _config.ForecasterPort,
	}
}

//////////////////// PUBLIC FUNCTIONS ////////////////////

// Function to run the http server
func RunHttpServer() error {
	http.HandleFunc("/healthz", healthzHandler)

	ip := _config.HttpServerHost
	port := strconv.FormatUint(uint64(_config.HttpServerPort), 10)
	err := http.ListenAndServe(ip + ":" + port, nil)

	return err
}

//////////////////// PRIVATE REQUEST HANDLERS FUNCTIONS ////////////////////

// Function to handle requests to "/healthz" endpoint. 
// This endpoint is useful to check if the DFaaS agent is healthy, and also if other main components (Forecaster and OpenFaaS cluster) are healthy.
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "DFaaS Node running.\n")
	io.WriteString(w, "Components status:\n")

	// Check Forecaster status
	_, err := _forecasterClient.HealthCheck()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, "- DFaaS Forecaster not ready.\n")
	} else {
		io.WriteString(w, "- DFaaS Forecaster ready.\n")
	}

	// Check OpenFaaS status
	respStatusOpenFaaS, err := healthCheckOpenFaaS()
	if err != nil || respStatusOpenFaaS != "200 OK" {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, "- OpenFaaS cluster not ready.\n")
	} else {
		io.WriteString(w, "- OpenFaaS cluster ready.\n")
	}
}

// Function used by the "healthzHandler" to send a request to the "/healthz" endpoint of OpenFaaS.
func healthCheckOpenFaaS() (string, error) {
	strURL := fmt.Sprintf("http://%s:%d/healthz", _config.OpenFaaSHost, _config.OpenFaaSPort)
	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", strURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return resp.Status, nil
}
