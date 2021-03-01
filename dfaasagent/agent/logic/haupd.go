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
	recalcPeriod time.Duration,
	entries map[string]*nodestbl.Entry,
	funcLimits map[string]uint,
) *HACfg {
	hacfg := &HACfg{
		MyNodeID:     myNodeID,
		OpenFaaSHost: openFaaSHost,
		OpenFaaSPort: openFaaSPort,

		StrRecalc:  recalcPeriod.String(),
		SecsRecalc: uint(recalcPeriod / time.Second),

		Nodes:     map[string]*HACfgNode{},
		Functions: map[string]*HACfgFunc{},
	}

	for funcName, limit := range funcLimits {
		hacfg.Functions[funcName] = &HACfgFunc{
			Limit:    limit,
			Weights:  map[string]uint{},
			LimitsIn: map[string]uint{},
		}
	}

	for nodeID, entry := range entries {
		hacfg.Nodes[nodeID] = &HACfgNode{
			HAProxyHost: entry.HAProxyHost,
			HAProxyPort: entry.HAProxyPort,
		}

		for funcName, funcData := range entry.FuncsData {
			_, present := hacfg.Functions[funcName]

			if !present {
				// We do not have this function on our OpenFaaS
				hacfg.Functions[funcName] = &HACfgFunc{
					Limit:    0,
					Weights:  map[string]uint{},
					LimitsIn: map[string]uint{},
				}
			}

			hacfg.Functions[funcName].Weights[nodeID] = funcData.NodeWeight
			hacfg.Functions[funcName].LimitsIn[nodeID] = uint(funcData.LimitIn)
		}
	}

	return hacfg
}

// updateHAProxyConfig updates the HAProxy config file with the provided object.
// Note: Go templates sort maps by key (see https://stackoverflow.com/questions/33860961/sorted-map-iteration-in-go-templates#comment76560806_33862260)
func updateHAProxyConfig(hacfg *HACfg) error {
	hacfg.Now = time.Now()

	return _hacfgupdater.UpdateHAConfig(hacfg)
}
