package logic

import (
	"time"

	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
)

// Note: to monitor the content of the haproxy.cfg file in bash:
// watch -n0.2 "cat dfaasvolume1/haproxy.cfg | column"

//////////////////// TYPES FOR HAPROXY CONFIG UPDATE ////////////////////

// HACfgNode contains the info about a node for the HAProxy config file
type HACfgNode struct {
	HAProxyHost string
	HAProxyPort uint
}

// HACfgFunc contains the info about a function for the HAProxy config file
type HACfgFunc struct {
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

// HACfg is the main struct for writing the HAProxy config file
type HACfg struct {
	// This is set by the updateHAProxyConfig function at the moment of writing
	Now time.Time

	MyNodeID     string
	OpenFaaSHost string
	OpenFaaSPort uint

	HttpServerHost string
	HttpServerPort uint

	// String representation of RecalPeriod (e.g. "5s")
	StrRecalc string
	// Number of seconds of RecalPeriod
	SecsRecalc uint

	// The key is the node ID
	Nodes map[string]*HACfgNode

	// The key is the function name
	Functions map[string]*HACfgFunc
}

//////////////////// FUNCTIONS FOR HAPROXY CONFIG UPDATE ////////////////////

func createHACfgObject(
	myNodeID string,
	openFaaSHost string,
	openFaaSPort uint,
	httpServerHost string,
	httpServerPort uint,
	recalcPeriod time.Duration,
	entries map[string]*nodestbl.Entry,
	funcLimits map[string]uint,
) *HACfg {
	hacfg := &HACfg{
		MyNodeID:     myNodeID,
		OpenFaaSHost: openFaaSHost,
		OpenFaaSPort: openFaaSPort,

		HttpServerHost: httpServerHost,
		HttpServerPort: httpServerPort,

		StrRecalc:  recalcPeriod.String(),
		SecsRecalc: uint(recalcPeriod / time.Second),

		Nodes:     map[string]*HACfgNode{},
		Functions: map[string]*HACfgFunc{},
	}

	// For each function in this node set limit (maxrate).
	for funcName, limit := range funcLimits {
		hacfg.Functions[funcName] = &HACfgFunc{
			Limit:    limit,
			Weights:  map[string]uint{},
			LimitsIn: map[string]uint{},
		}
	}

	// For each node write Host and port.
	for nodeID, entry := range entries {
		hacfg.Nodes[nodeID] = &HACfgNode{
			HAProxyHost: entry.HAProxyHost,
			HAProxyPort: entry.HAProxyPort,
		}

		// For each function on selected node check if this function
		// is also present in that node.
		// If it is not present on this node set limit to 0 and
		// weight and Limit in empty.
		for funcName, funcData := range entry.FuncsData {
			_, present := hacfg.Functions[funcName]

			if !present {
				// We do not have this function on our OpenFaaS.
				// Set limit (maxrate) to 0 --> can't accept request for
				// this function in local FaaS cluster, but can
				// fwd to another node.
				hacfg.Functions[funcName] = &HACfgFunc{
					Limit:    0,
					Weights:  map[string]uint{},
					LimitsIn: map[string]uint{},
				}
			}

			// For the function funcName in nodeID set weights and LimitsIn written
			// previously during the 2 phases of racalc algo.
			hacfg.Functions[funcName].Weights[nodeID] = funcData.NodeWeight
			hacfg.Functions[funcName].LimitsIn[nodeID] = uint(funcData.LimitIn) // Note: Seems that is not written in template file.

			// Weight are used in template file for load-balancing, with custom
			// weights toward other nodes of p2p net.
			// See: https://serverfault.com/questions/113637/haproxy-roundrobin-weights
			// Also doc: http://cbonte.github.io/haproxy-dconv/configuration-1.4.html#5-weight
		}
	}

	return hacfg
}

// updateHAProxyConfig updates the HAProxy config file with the provided object.
// Note: Go templates sort maps by key (see https://stackoverflow.com/questions/33860961/sorted-map-iteration-in-go-templates#comment76560806_33862260)
func updateHAProxyConfig(hacfg *HACfg) error {
	hacfg.Now = time.Now()

	// hacfg is a struct of type HACfg, that is use as content
	// for writing template file.
	// Indeed, in template file is possible to access fields
	// of this struct (see file of template .tmpl).
	// Template is referred to a data structure, in that case HACfg type.
	// See https://golang.org/pkg/text/template/.
	return _hacfgupdater.UpdateHAConfig(hacfg)
}
