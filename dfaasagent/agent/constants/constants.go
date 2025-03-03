// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package contains only some constants
package constants

const (
	// HAProxyMaxWeight is the maximum possible weight value that should be used
	// in the HAProxy configuration file
	HAProxyMaxWeight = 100

	// Names of the different strategies supported by the DFaaS agent
	RecalcStrategy = "recalcstrategy"
	NodeMarginStrategy = "nodemarginstrategy"
)
