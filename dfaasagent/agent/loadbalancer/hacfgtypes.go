// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"time"
)

//////////////////// TYPES FOR HAPROXY CONFIG UPDATE ////////////////////

// Base struct representing the basic information for for writing the HAProxy config file
type HACfg struct {
	// This is set by the updateHAProxyConfig function at the moment of writing
	Now time.Time

	MyNodeID    string
	NodeIP      string
	HAProxyHost string
	FaaSHost    string
	FaaSPort    uint
}

// HACfgNodeNMS contains the info about a node for the HAProxy config file.
type HACfgNodeNMS struct {
	HAProxyHost string
	HAProxyPort uint
}

// HACfgFuncNMS contains the weights info about a function for the HAProxy
// config file.
type HACfgFuncNMS struct {
	// Weights of the nodes. The key is the node ID
	Weights map[string]uint
}

// HACfgNMS represents the information specific for Node Margin strategy
// for writing the HAProxy config file
type HACfgNMS struct {
	HACfg

	// String representation of RecalPeriod (e.g. "5s")
	StrRecalc string

	// The key is the node ID
	Nodes map[string]*HACfgNodeNMS

	// The key is the function name
	Functions map[string]*HACfgFuncNMS
}

/////////////////// HACFG TYPES FOR STATIC STRATEGY ///////////////////

// HACfgNodeStatic contains the info about a node for the HAProxy config file.
type HACfgNodeStatic struct {
	HAProxyHost string
	HAProxyPort uint
}

// HACfgFuncNMS contains the weights info about a function for the HAProxy
// config file.
type HACfgFuncStatic struct {
	// Weights of the nodes. The key is the node ID.
	Weights map[string]uint
}

// HACfgNMS represents the information specific for Static strategy for writing
// the HAProxy config file.
type HACfgStatic struct {
	HACfg

	// The key is the node ID.
	Nodes map[string]*HACfgNodeStatic

	// The key is the function name.
	Functions map[string]*HACfgFuncStatic
}
