// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or later license. See LICENSE and
// AUTHORS file for more information.

// Package faasprovider defines the FaaSProvider interface, which abstracts the
// underlying FaaS platform (e.g. OpenFaaS, OpenWhisk) and its associated
// Prometheus metrics queries so that the rest of the agent remains
// platform-agnostic.
package faasprovider

import (
	"fmt"
    "time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider/openfaas"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider/openwhisk"

)

const (
	// PlatformOpenFaaS is the identifier for the OpenFaaS platform.
	PlatformOpenFaaS = "openfaas"

	// PlatformOpenWhisk is the identifier for the OpenWhisk platform.
	PlatformOpenWhisk = "openwhisk"
)

// FaaSProvider is the abstraction layer between the DFaaS agent and a concrete
// FaaS platform. Implementations must provide access to deployed function
// metadata, platform-specific Prometheus metrics, platform-agnostic node
// metrics (node-exporter / cAdvisor), and a gateway health check.
type FaaSProvider interface {
	// GetFuncsWithMaxRates returns deployed function names mapped to their
	// dfaas.maxrate value (max requests/s). Used by RecalcStrategy.
	GetFuncsWithMaxRates() (map[string]uint, error)

	// GetFuncsNames returns the list of deployed function names. Used by
	// StaticStrategy and NodeMarginStrategy.
	GetFuncsNames() ([]string, error)

	// GetFuncsWithTimeout returns deployed function names mapped to their
	// dfaas.timeout_ms value in ms, or nil if the label is absent. Used by
	// AllLocalStrategy.
	GetFuncsWithTimeout() (map[string]*uint, error)

	// QueryAFET returns the Average Function Execution Time (seconds) per
	// function over the given time span.
	QueryAFET(timeSpan time.Duration) (map[string]float64, error)

	// QueryInvoc returns the invocation rate per function and per status code
	// over the given time span. Outer key: function name. Inner key: status
	// code string ("200", "500", ...).
	QueryInvoc(timeSpan time.Duration) (map[string]map[string]float64, error)

	// QueryServiceCount returns the number of running replicas per function.
	QueryServiceCount() (map[string]int, error)

	// QueryCPUusage returns CPU usage percentage per node-exporter instance.
	QueryCPUusage(timeSpan time.Duration) (map[string]float64, error)

	// QueryRAMusage returns RAM usage percentage per node-exporter instance.
	QueryRAMusage(timeSpan time.Duration) (map[string]float64, error)

	// QueryCPUusagePerFunction returns CPU usage percentage per function
	// container.
	QueryCPUusagePerFunction(timeSpan time.Duration, funcNames []string) (map[string]float64, error)

	// QueryRAMusagePerFunction returns RAM usage percentage per function
	// container.
	QueryRAMusagePerFunction(timeSpan time.Duration, funcNames []string) (map[string]float64, error)

	// HealthCheck returns "200 OK" if the FaaS gateway is reachable, or an
	// error otherwise.
	HealthCheck() (string, error)

    // Platform returns the FaaS platform name ("openfaas", "openwhisk"...).
    Platform() string
}


// BackendPathPrefix returns the URL path prefix used to invoke a function
// for the given platform and namespace.
//
// For OpenFaaS (default): "/function"
// For OpenWhisk: "/api/v1/namespaces/<namespace>/actions"
func BackendPathPrefix(platform, namespace string) string, error {
    switch platform {
    case "", PlatformOpenFaaS:
        return "/function", nil
    case PlatformOpenWhisk:
		if namespace == "" {
			namespace = "guest"
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/actions", namespace), nil
	default:
		return "", fmt.Errorf("unknown FaaS platform %q", platform)
}

// NewFaaSProvider returns the FaaSProvider for the given platform. host and
// port are the FaaS gateway coordinates (used for both platforms). namespace
// and apiKey are only used for OpenWhisk.
func NewFaaSProvider(platform, host string, port uint, namespace, apiKey string) (FaaSProvider, error) {
	switch platform {
	case "", PlatformOpenFaaS:
		return openfaas.New(host, port), nil
	case PlatformOpenWhisk:
		return openwhisk.New(
			fmt.Sprintf("%s:%d", host, port),
			namespace,
			apiKey,
		), nil
	default:
		return nil, fmt.Errorf("unknown FaaS platform %q", platform)
	}
}
