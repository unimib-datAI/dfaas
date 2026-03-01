// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or later license. See LICENSE and
// AUTHORS file for more information.

package faasprovider

import (
	"fmt"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider/openfaas"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider/openwhisk"
)

const (
	// PlatformOpenFaaS is the identifier for the OpenFaaS platform.
	PlatformOpenFaaS = "openfaas"
	// PlatformOpenWhisk is the identifier for the OpenWhisk platform.
	PlatformOpenWhisk = "openwhisk"
)

// NewFaaSProvider returns the FaaSProvider for the given platform.
// host and port are the FaaS gateway coordinates (used for both platforms).
// namespace and apiKey are only used for OpenWhisk.
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
		return nil, fmt.Errorf("unknown AGENT_FAAS_PLATFORM %q; valid values: %q, %q",
			platform, PlatformOpenFaaS, PlatformOpenWhisk)
	}
}
