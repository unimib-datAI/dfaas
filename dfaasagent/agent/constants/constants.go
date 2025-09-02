// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package contains only some constants
package constants

const (
	// Names of the different strategies supported by the DFaaS agent
	RecalcStrategy     = "recalcstrategy"
	NodeMarginStrategy = "nodemarginstrategy"

	// HAProxyMaxWeight is the maximum possible weight value that should be used
	// in the HAProxy configuration file
	HAProxyMaxWeight = 100

	// Connection information for accessing the Data Plane API.
	HAProxyDataPlaneAPIOrigin = "http://haproxy.default.svc.cluster.local:5555"
	HAProxyDataPlaneUsername  = "admin"
	HAProxyDataPlanePassword  = "admin"

	// Connection information for accessing the DFaaS Forecaster service.
	ForecasterHost = "dfaas-forecaster"
	ForeasterPort  = 80

	// Connection information used to run an HTTP Web Server for health checking
	// at /healthz endpoint.
	HttpServerHost = "0.0.0.0"
	HttpServerPort = 8080
)
