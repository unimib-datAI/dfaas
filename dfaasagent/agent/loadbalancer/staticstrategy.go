// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/constants"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/httpserver"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

// StaticStrategy implements the Static strategy, where the local node always
// processes 60% of incoming requests and forwards 40% to neighbors, divided
// evenly among them. If there are no neighbors, all requests are processed
// locally.
type StaticStrategy struct {
	hacfgupdater  hacfgupd.Updater
	nodestbl      *nodestbl.TableNMS
	offuncsClient *offuncs.Client

	nodeInfo nodeInfoStatic
	// Map of target nodes, with node ID of a common neighbour as key,
	// and array of common functions as value
	targetNodes map[string][]string
	// Map containing for each node's function (key) another map containing
	// for each node the corresponding forwarding weight
	weights map[string]map[string]uint
}

// Private struct containing info about us.
type nodeInfoStatic struct {
	funcs               []string // Our OpenFaaS functions.
	commonNeighboursNum int      // Number of neighbours with at least a function in common.
}

// RunStrategy handles the periodic execution of the recalculation function. It
// should run in a goroutine.
func (strategy *StaticStrategy) RunStrategy() error {
	logger := logging.Logger()

	var millisNow, millisSleep int64
	var err error

	millisInterval := int64(_config.RecalcPeriod / time.Millisecond)

	for {
		if err := strategy.publishNodeInfo(); err != nil {
			logger.Error("Failed to publish node info, skipping RunStrategy iteration ", err)
			logger.Warn("Waiting 5 second before retrying RunStrategy")
			time.Sleep(5 * time.Second)
			continue
		}

		strategy.updateCommonNeighbours()

		strategy.weights, err = strategy.calculateWeights()
		if err != nil {
			return fmt.Errorf("calculating new weights: %w", err)
		}

		if err = strategy.setProxyWeights(); err != nil {
			return fmt.Errorf("setting new weights: %w", err)
		}

		// TODO: Change the counter instance.
		httpserver.NmsSuccessIterations.Inc()

		millisNow = time.Now().UnixNano() / 1000000
		millisSleep = millisInterval - (millisNow % millisInterval)
		time.Sleep(time.Duration(millisSleep) * time.Millisecond)
	}
}

// OnReceived is executed every time a message from a peer is received.
func (strategy *StaticStrategy) OnReceived(msg *pubsub.Message) error {
	// Check the type before deserializing to the right object.
	var msgForType struct{ MsgType string }
	if err := json.Unmarshal(msg.GetData(), &msgForType); err != nil {
		return fmt.Errorf("deserializing PubSub msg: %w", err)
	}

	if msgForType.MsgType != StrMsgNodeInfoTypeStatic {
		logging.Logger().Warnf("Received a PubSub message of type %q, expected %q",
			msgForType.MsgType,
			StrMsgNodeInfoTypeStatic)
		return nil
	}

	var objMsg MsgNodeInfoStatic
	if err := json.Unmarshal(msg.GetData(), &objMsg); err != nil {
		return fmt.Errorf("deserializing PubSub msg: %w", err)
	}

	return strategy.processMsgNodeInfoStatic(msg.GetFrom().String(), &objMsg)
}

// In Static strategy, 60% of requests are handled locally, 40% divided among
// neighbours.
func (strategy *StaticStrategy) calculateWeights() (map[string]map[string]uint, error) {
	myNodeID := _p2pHost.ID().String()
	weights := make(map[string]map[string]uint)

	// Get current common neighbours.
	var neighbourIDs []string
	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		for neighID, entry := range entries {
			if entry.CommonNeighbour {
				neighbourIDs = append(neighbourIDs, neighID)
			}
		}
		return nil
	})

	numNeighbours := len(neighbourIDs)
	localWeight := float64(constants.HAProxyMaxWeight) * 0.6
	neighbourWeight := float64(constants.HAProxyMaxWeight) * 0.4

	// Set the weights for each deployed functions.
	for _, funcName := range strategy.nodeInfo.funcs {
		weights[funcName] = make(map[string]uint)
		weights[funcName][myNodeID] = uint(math.Round(localWeight))

		if numNeighbours > 0 {
			// Divide the 40% equally among neighbours.
			perNeigh := neighbourWeight / float64(numNeighbours)
			for _, neighID := range neighbourIDs {
				weights[funcName][neighID] = uint(math.Round(perNeigh))
			}
		} else {
			// No neighbours, everything local.
			weights[funcName][myNodeID] = constants.HAProxyMaxWeight
		}
	}

	debugWeightsNMS(weights)

	return weights, nil
}

// Send to other nodes our information.
func (strategy *StaticStrategy) publishNodeInfo() error {
	var err error

	// Obtain our function names list.
	strategy.nodeInfo.funcs, err = strategy.offuncsClient.GetFuncsNames()
	if err != nil {
		return fmt.Errorf("getting functions info from OpenFaaS: %w", err)
	}

	msg := MsgNodeInfoStatic{
		MsgType:     StrMsgNodeInfoTypeStatic,
		HAProxyHost: _config.NodeIP,
		HAProxyPort: constants.HAProxyPort,
		Functions:   strategy.nodeInfo.funcs,
	}
	debugMsgNodeInfoStatic(msg)

	err = communication.MarshAndPublish(msg)
	if err != nil {
		return fmt.Errorf("publishing node info to other DFaaS nodes: %w", err)
	}

	return nil

	// Obtain our function names list
	strategy.nodeInfo.funcs, err = strategy.offuncsClient.GetFuncsNames()
	if err != nil {
		return err
	}

	return nil
}

// Check which neighbour has at least a function in common and update the
// relative information in nodestbl.
func (strategy *StaticStrategy) updateCommonNeighbours() {
	commonNeighbours := 0
	var targetNodes = make(map[string][]string)
	var common bool

	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		for neighbourID, neighInfo := range entries {
			common = false
			var commonFuncs []string
			for i := 0; i < len(strategy.nodeInfo.funcs); i++ {
				for j := 0; j < len(neighInfo.Funcs); j++ {
					if strategy.nodeInfo.funcs[i] == neighInfo.Funcs[j] {
						commonFuncs = append(commonFuncs, strategy.nodeInfo.funcs[i])
						common = true
						break
					}
				}
			}
			if common {
				commonNeighbours += 1
				neighbour, _ := entries[neighbourID]
				neighbour.CommonNeighbour = true
				entries[neighbourID] = neighbour
				targetNodes[neighbourID] = commonFuncs
			}
		}

		// Update targetNodes
		strategy.targetNodes = targetNodes
		// Update commonNeighboursNum
		strategy.nodeInfo.commonNeighboursNum = commonNeighbours

		return nil
	})
}

// Update HAProxy configuration with new weights.
func (strategy *StaticStrategy) setProxyWeights() error {
	myID := _p2pHost.ID().String()

	var hacfg *HACfgStatic
	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		hacfg = strategy.createHACfgObject(
			myID,
			_config.OpenFaaSHost,
			_config.OpenFaaSPort,
			entries,
			strategy.weights,
		)
		return nil
	})

	hacfg.Now = time.Now()
	if err := strategy.hacfgupdater.UpdateHAConfig(hacfg); err != nil {
		return err
	}
	return nil
}

// Method which creates and returns the HACfgStatic object, used from method
// updateHAProxyConfig to update the HAProxy configuration.
func (strategy *StaticStrategy) createHACfgObject(
	myNodeID string,
	openFaaSHost string,
	openFaaSPort uint,
	entries map[string]*nodestbl.EntryNMS,
	funcsWeights map[string]map[string]uint,
) *HACfgStatic {
	hacfg := &HACfgStatic{
		HACfg: HACfg{
			MyNodeID:     myNodeID,
			NodeIP:       _config.NodeIP,
			HAProxyHost:  _config.HAProxyHost,
			OpenFaaSHost: openFaaSHost,
			OpenFaaSPort: openFaaSPort,
		},

		Nodes:     map[string]*HACfgNodeStatic{},
		Functions: map[string]*HACfgFuncStatic{},
	}

	// For each node write Host and port.
	for nodeID, entry := range entries {
		hacfg.Nodes[nodeID] = &HACfgNodeStatic{
			HAProxyHost: entry.HAProxyHost,
			HAProxyPort: entry.HAProxyPort,
		}
	}

	// For each function write weights for load-balancing.
	for funcName, weights := range funcsWeights {
		hacfg.Functions[funcName] = &HACfgFuncStatic{
			Weights: weights,
		}
	}

	return hacfg
}

// processMsgNodeInfoStatic processes a node info message received from pubsub.
func (strategy *StaticStrategy) processMsgNodeInfoStatic(sender string, msg *MsgNodeInfoStatic) error {
	logger := logging.Logger()

	if sender == _p2pHost.ID().String() {
		return nil // Ignore ourselves.
	}

	if logging.GetDebugMode() {
		logger.Debugf("Received msg from node: %s (%s:%d) (functions: %v)",
			sender, msg.HAProxyHost, msg.HAProxyPort, msg.Functions)
	}

	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		_, present := entries[sender]
		if !present {
			entries[sender] = &nodestbl.EntryNMS{
				CommonNeighbour: false,
				Load:            nodestbl.Load{},
			}
			logger.Debugf("Node %s was not present and has been added to the table", sender)
		}
		entries[sender].TAlive = time.Now()
		entries[sender].HAProxyHost = msg.HAProxyHost
		entries[sender].HAProxyPort = msg.HAProxyPort
		entries[sender].Funcs = msg.Functions

		return nil
	})

	return nil
}
