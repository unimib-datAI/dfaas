// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package is for handling the information about the other nodes in the
// network, such as the max rate limits and the address and port of the relative
// HAProxy server instance
package nodestbl

import (
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/config"
)

//////////////////// MAIN PRIVATE VARS AND INIT FUNCTION ////////////////////

var _config config.Configuration

// Initialize initializes this package
func Initialize(config config.Configuration) {
	_config = config
}
