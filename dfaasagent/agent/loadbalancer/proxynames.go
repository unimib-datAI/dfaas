// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"fmt"
	"text/template"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faas"
)

// This file contains helpers functions to be called inside the HAProxy
// templates.

// funcBackend returns the name of the HAProxy backend for client requests to
// the given function.
func funcBackend(funcName string) string {
	return "function_" + faas.ProxyID(funcName)
}

// funcForwardedBackend returns the name of the HAProxy backend for requests
// to the given function forwarded by the given neighbor.
func funcForwardedBackend(funcName, neighborID string) string {
	return fmt.Sprintf("function_%s_forwarded_%s", faas.ProxyID(funcName), neighborID)
}

// templateFuncs returns the functions available to HAProxy config templates
// based on the FaaS platform used:
//
//   - proxyID: returns the function name to use in HAProxy identifiers,
//   - funcPath: returns the URL path used by clients to invoke a function,
//   - funcsPathPrefix: returns the URL path prefix shared by all functions,
//   - faasPlatform: returns name of the FaaS platform.
func templateFuncs(provider faas.Provider) template.FuncMap {
	return template.FuncMap{
		"proxyID":         faas.ProxyID,
		"funcPath":        provider.Path,
		"funcsPathPrefix": provider.PathPrefix,
		"faasPlatform":    provider.Platform,
	}
}
