// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package handles a web server to expose endpoints on the agent (e.g.
// endpoint for DFaaS node healthcheck)
package httpserver

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/config"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/constants"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/forecaster"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//////////////////// MAIN PRIVATE VARS AND INIT FUNCTION ////////////////////

var _config config.Configuration
var _faasProvider faasprovider.FaaSProvider
var _forecasterClient forecaster.Client

var StrategySuccessIterations = promauto.NewCounter(prometheus.CounterOpts{
	Name: "dfaas_agent_strategy_iterations",
	Help: "The total number of successfully strategy iterations.",
})

var StrategyIterationDuration = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "dfaas_agent_strategy_iteration_duration_seconds",
	Help: "Execution duration of a successfully strategy iteration in seconds",
})

// Initialize initializes this package (sets some vars, etc...)
func Initialize(config config.Configuration, provider faasprovider.FaaSProvider) {
	_config = config

	_forecasterClient = forecaster.Client{
		Hostname: constants.ForecasterHost,
		Port:     constants.ForeasterPort,
	}

	_faasProvider = provider
}

//////////////////// PUBLIC FUNCTIONS ////////////////////

// Function to run the http server
func RunHttpServer() error {
	// Expose to Prometheus only custom metrics by creating a new registry.
	customRegistry := prometheus.NewRegistry()

	customRegistry.MustRegister(StrategySuccessIterations)
	customRegistry.MustRegister(StrategyIterationDuration)

	http.HandleFunc("/healthz", healthzHandler)
	http.Handle("/metrics", promhttp.HandlerFor(customRegistry, promhttp.HandlerOpts{}))

	ip := constants.HttpServerHost
	port := strconv.FormatUint(uint64(constants.HttpServerPort), 10)
	err := http.ListenAndServe(ip+":"+port, nil)

	return err
}

//////////////////// PRIVATE REQUEST HANDLERS FUNCTIONS ////////////////////

// Function to handle requests to "/healthz" endpoint.
//
// This endpoint is useful to check if the DFaaS agent is healthy, and also if
// other main components (Forecaster and FaaS gateway) are healthy.
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

	// Check FaaS gateway status.
	prefixMsg := fmt.Sprintf("- FaaS gateway (%s)", _faasProvider.Platform())
	respStatusFaaS, err := _faasProvider.HealthCheck()
	if err != nil || respStatusFaaS != "200 OK" {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, prefixMsg+" not ready.\n")
	} else {
		io.WriteString(w, prefixMsg+" ready.\n")
	}
}
