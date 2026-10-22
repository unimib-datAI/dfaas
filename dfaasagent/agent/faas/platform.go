// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or later license. See LICENSE and
// AUTHORS file for more information.

// Package faas defines the Provider interface, which abstracts some methods for
// obtaining information from the underlying FaaS platform (e.g., OpenFaaS,
// OpenWhisk). Not everything is abstracted, which is why the underlying FaaS
// platform structures are also exported.
package faas

import (
	"fmt"
)

// Currently supported FaaS platforms.
const (
	PlatformOpenFaaS  = "openfaas"
	PlatformOpenWhisk = "openwhisk"
)

type Provider interface {
	// Platform returns the underlying FaaS platform name. Could be
	// [PlatformOpenFaaS] or [PlatformOpenWhisk].
	Platform() string

	// FuncsNames returns the list of deployed function names.
	Names() ([]string, error)

	// WithMaxRates returns deployed function names mapped to their
	// dfaas.maxrate value (max requests/s).
	WithMaxRates() (map[string]uint, error)

	// WithTimeout returns deployed function names mapped to their
	// dfaas.timeout_ms value in ms, or nil if the label is absent.
	WithTimeout() (map[string]*uint, error)

	// HealthCheck returns "200 OK" if the FaaS platform is reachable, or an
	// error otherwise.
	HealthCheck() (string, error)
}

// New returns the Provider for the given platform.
//
// host and port are the FaaS gateway coordinates (used for both platforms).
// namespace and apiKey are only used for OpenWhisk.
func New(platform, host string, port uint, namespace, apiKey string) (Provider, error) {
	switch platform {
	case "", PlatformOpenFaaS:
		return NewOpenFaaS(host, port)
	case PlatformOpenWhisk:
		return NewOpenWhisk(host, port, namespace, apiKey)
	default:
		return nil, fmt.Errorf("unsupported FaaS platform %q", platform)
	}
}
