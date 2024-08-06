package loadbalancer

import (
	"time"
)

// Note: to monitor the content of the haproxy.cfg file in bash:
// watch -n0.2 "cat dfaasvolume1/haproxy.cfg | column"

//////////////////// TYPES FOR HAPROXY CONFIG UPDATE ////////////////////

// Base struct representing the basic information for for writing the HAProxy config file
type HACfg struct {
	// This is set by the updateHAProxyConfig function at the moment of writing
	Now time.Time

	MyNodeID     string
	OpenFaaSHost string
	OpenFaaSPort uint
}

/////////////////// HACFG TYPES FOR RECALC STRATEGY ///////////////////

// HACfgNodeRecalc contains the info about a node for the HAProxy config file
type HACfgNodeRecalc struct {
	HAProxyHost string
	HAProxyPort uint
}

// HACfgFuncRecalc contains the info about a function for the HAProxy config file
type HACfgFuncRecalc struct {
	// Max limit of req/s for invocations on this node's OpenFaaS instance.
	// Should be = 0 if we do not have the function on our local OpenFaaS
	// cluster
	Limit uint

	// Weight of the nodes. The key is the node ID
	Weights map[string]uint
	// Limits (in req/s) for incoming requests from each node. The key is the
	// node ID
	LimitsIn map[string]uint
}

// HACfgRecalc represents the information specific for Recalc strategy
// for writing the HAProxy config file
type HACfgRecalc struct {
	HACfg

	HttpServerHost string
	HttpServerPort uint

	// String representation of RecalPeriod (e.g. "5s")
	StrRecalc string
	// Number of seconds of RecalPeriod
	SecsRecalc uint

	// The key is the node ID
	Nodes map[string]*HACfgNodeRecalc

	// The key is the function name
	Functions map[string]*HACfgFuncRecalc
}


/////////////////// HACFG TYPES FOR RECALC STRATEGY ///////////////////

// HACfgNodeNMS contains the info about a node for the HAProxy config file
type HACfgNodeNMS struct {
	HAProxyHost string
	HAProxyPort uint
}

// HACfgFuncNMS contains the weights info about a function for the HAProxy config file
type HACfgFuncNMS struct {
	// Weights of the nodes. The key is the node ID
	Weights map[string]uint
}

// HACfgNMS represents the information specific for Node Margin strategy
// for writing the HAProxy config file
type HACfgNMS struct {
	HACfg

	HttpServerHost string
	HttpServerPort uint

	// The key is the node ID
	Nodes map[string]*HACfgNodeNMS

	// The key is the function name
	Functions map[string]*HACfgFuncNMS
}
