// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/constants"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/httpserver"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/hasock"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/utils/p2phostutils"
)

// This file contains the implementation of the Recalc strategy.

// Struct representing a RecalcStrategy instance, which implements the Strategy interface

// RecalcStrategy represent a Recalc strategy instance.
//
// As it implements the Strategy interface, it has the RunStrategy() and
// OnReceived() methods.
//
// As an internal detail, the strategy action is divided into two steps.
type RecalcStrategy struct {
	hacfgupdater  hacfgupd.Updater
	nodestbl      *nodestbl.TableRecalc
	offuncsClient *offuncs.Client

	// The following variables are specific to the Recalc algorithm.
	nodeIDs   []peer.ID          // IDs of the connected p2p nodes
	funcs     map[string]uint    // Our OpenFaaS functions with dfaas.maxrate limits
	userRates map[string]float64 // Invocation rates for users only (in req/s) (from HAProxy stick-tables)

	// For each function, the value is true if the node is currently in overload
	// mode (req/s >= maxrate), false if underload
	overloads map[string]bool

	it int // = 0 // Number of agent loop iterations
}

// RunStrategy handles the periodic execution of the recalculation function. It
// should run in a goroutine.
func (strategy *RecalcStrategy) RunStrategy() error {
	logger := logging.Logger()

	var millisNow, millisSleep int64

	if _config.RecalcPeriod == 0 {
		logger.Warn("Given RecalcPeriod must be a positive time duration, using 1 minute by default")
		_config.RecalcPeriod = 1 * time.Minute
	}

	// Set the interval to wait after a failed recalculation attempt. This is
	// used for both step 1 and step 2 error recovery.
	failedInterval := 5 * time.Second

	// Calculate the interval (in milliseconds) at which the recalculation
	// should occur.
	millisInterval := int64(_config.RecalcPeriod / time.Millisecond)

	// Calculate half of the interval (in milliseconds). Used for timing the
	// second recalculation step to occur halfway between intervals.
	millisIntervalHalf := millisInterval / 2

	// The Recalc strategy code is divided into two steps: the first gathers
	// data and updates the local state, and the second calculates the new
	// weights and updates the HAProxy configuration.
	for {
		millisNow = time.Now().UnixNano() / 1000000
		millisSleep = millisInterval - (millisNow % millisInterval)
		time.Sleep(time.Duration(millisSleep) * time.Millisecond)

		if err := strategy.recalcStep1(); err != nil {
			logger.Error("Failed Recalc step 1, skipping RunStrategy iteration ", err)
			logger.Warnf("Waiting %v before retrying RunStrategy", failedInterval.Seconds())
			time.Sleep(failedInterval)
			continue
		}

		millisNow = time.Now().UnixNano() / 1000000
		millisSleep = millisInterval - ((millisNow + millisIntervalHalf) % millisInterval)
		time.Sleep(time.Duration(millisSleep) * time.Millisecond)

		if err := strategy.recalcStep2(); err != nil {
			logger.Error("Failed Recalc step 2, skipping RunStrategy iteration ", err)
			logger.Warnf("Waiting %v before retrying RunStrategy", failedInterval.Seconds())
			time.Sleep(failedInterval)
			continue
		}

		httpserver.StrategySuccessIterations.Inc()
	}
}

// OnReceived is executed every time a message from a peer is received.
func (strategy *RecalcStrategy) OnReceived(msg *pubsub.Message) error {
	var msgForType struct{ MsgType string }
	if err := json.Unmarshal(msg.GetData(), &msgForType); err != nil {
		return fmt.Errorf("Error while deserializing a message from the PubSub subscription: %w", err)
	}

	switch msgForType.MsgType {
	case StrMsgTextType:
		var objMsg MsgText
		if err := json.Unmarshal(msg.GetData(), &objMsg); err != nil {
			return fmt.Errorf("Error while deserializing a message from the PubSub subscription: %w", err)
		}

		processMsgText(msg.GetFrom().String(), &objMsg)
	case StrMsgNodeInfoTypeRecalc:
		var objMsg MsgNodeInfoRecalc
		if err := json.Unmarshal(msg.GetData(), &objMsg); err != nil {
			return fmt.Errorf("Error while deserializing a message from the PubSub subscription: %w", err)
		}

		strategy.processMsgNodeInfoRecalc(msg.GetFrom().String(), &objMsg)
	default:
		logging.Logger().Warnf("Unrecognized message type %q", msgForType.MsgType)
	}

	return nil
}

func (strategy *RecalcStrategy) recalcStep1() error {
	var err error
	logger := logging.Logger()

	// Get list of connected nodes.
	strategy.nodeIDs = p2phostutils.GetConnNodeIDsUniq(_p2pHost)
	debugConnectedNodes(strategy.nodeIDs)

	// Get stats about OpenFaaS functions.
	strategy.funcs, err = strategy.offuncsClient.GetFuncsWithMaxRates()
	if err != nil {
		return fmt.Errorf("get functions info from OpenFaaS: %w", err)
	}
	debugFuncs(strategy.funcs)

	// For each function, the following stick tables are defined in HAProxy:
	//
	//  - Per-function, per-user requests: st_users_func_<funcName>
	//    Purpose: Track invocation count and rate for each OpenFaaS function,
	//    only for requests from users (not other nodes).
	//
	//  - Per-function, local requests: st_local_func_<funcName>
	//    Purpose: Track invocation count and rate for each function handled by
	//    the local OpenFaaS instance.
	//
	//  - Per-function, per-node requests: st_other_node_<funcName>_<nodeID>
	//    Purpose: Track requests forwarded from other DFaaS nodes for each
	//    function, per node.
	//
	// These stick tables are used for rate limiting, forwarding logic, LimitIn
	// enforcement and tracking.
	//
	// Only st_users_func_<funcName> data are used to calculate weights!

	// Get stats from HAProxy stick tables (st_users_func_<funcName>).
	strategy.userRates = map[string]float64{}
	for funcName := range strategy.funcs {
		stName := fmt.Sprintf("st_users_func_%s", funcName)
		stContent, err := hasock.ReadStickTable(stName)

		if err != nil {
			logger.Error(fmt.Errorf("Error while reading the stick-table %q from the HAProxy socket: %w", stName, err))
			logger.Warnf("Not changing userRates for stick-table %q but this should be ok", stName)
			continue
		}

		for _, stEntry := range stContent {
			// There should be only one line, with key "80", which is the port
			// of the HAProxy frontend
			//
			// Note: the whole formula is multiplied by two at the end because
			// we know we restarted HAProxy at the end of recalcStep2
			strategy.userRates[funcName] = float64(stEntry.HTTPReqCnt) / float64(_config.RecalcPeriod/time.Second) * 2
		}

		debugStickTable(stName, stContent)
	}
	debugHAProxyUserRates(strategy.userRates)

	// Get stats from HAProxy stick tables (st_local_func_<funcName>).
	for funcName := range strategy.funcs {
		stName := fmt.Sprintf("st_local_func_%s", funcName)
		stContent, err := hasock.ReadStickTable(stName)

		if err != nil {
			logger.Error(fmt.Errorf("Error while reading the stick-table %q from the HAProxy socket: %w", stName, err))
			logger.Warnf("Not changing local rates for stick-table %q but this should be ok", stName)
			continue
		}

		debugStickTable(stName, stContent)
	}

	// Get stats for HAProxy stick tables (st_other_node_<funcName>_<nodeID>).
	for funcName := range strategy.funcs {
		for _, nodeID := range strategy.nodeIDs {
			stName := fmt.Sprintf("st_other_node_%s_%s", funcName, nodeID.String())
			stContent, err := hasock.ReadStickTable(stName)
			if err != nil {
				logger.Error(fmt.Errorf("Error while reading the stick-table %q from the HAProxy socket: %w", stName, err))
				logger.Warnf("Not changing other node rates for stick-table %q but this should be ok", stName)
				continue
			}
			debugStickTable(stName, stContent)
		}
	}

	// Set overload/underload state for each function.
	strategy.overloads = map[string]bool{}
	for funcName, maxRate := range strategy.funcs {
		invocRate, present := strategy.userRates[funcName]

		if !present || invocRate < float64(maxRate) {
			strategy.overloads[funcName] = false
		} else {
			strategy.overloads[funcName] = true
		}

		logger.Debugf("Function %q overloaded: %t", funcName, strategy.overloads[funcName])
	}

	// Calculate limits and weights.
	for funcName, overloaded := range strategy.overloads {
		if overloaded {
			logger.Debugf("Function %q is on overloaded! Setting LimitIn to 0", funcName)
			strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
				for _, entry := range entries {
					funcData, present := entry.FuncsData[funcName]
					if present {
						funcData.LimitIn = 0
					}
				}
				return nil
			})
			continue
		}

		// If not overloaded, we calculate the rate margin.
		invocRate, present := strategy.userRates[funcName]
		maxRate := strategy.funcs[funcName]
		var margin uint
		if present {
			margin = maxRate - uint(invocRate)
		} else {
			margin = maxRate
		}
		logger.Debugf("Function %q: invocation rate=%f max_rate=%d margin=%d", funcName, invocRate, maxRate, margin)

		strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
			nNodes := uint(0)
			for _, entry := range entries {
				funcData, present := entry.FuncsData[funcName]
				if present {
					funcData.NodeWeight = 0
					nNodes++
					logger.Debugf("Set Weight to 0 for %s function", funcName)
				}
			}
			if nNodes > 0 {
				limitIn := margin / nNodes
				for _, entry := range entries {
					funcData, present := entry.FuncsData[funcName]
					if present {
						funcData.LimitIn = float64(limitIn)
						logger.Debugf("Set LiminIn to %f for %s function", funcData.LimitIn, funcName)
					}
				}
			}
			return nil
		})
	}

	// Print content of NodeStbl.
	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
		debugNodesTblContent(entries)
		return nil
	})

	// Publish messages on p2p network.
	limits := map[string]map[string]float64{}
	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
		for _, nodeID := range strategy.nodeIDs {
			strNodeID := nodeID.String()
			entry, present := entries[strNodeID]
			if present {
				limits[strNodeID] = map[string]float64{}
				for funcName, funcData := range entry.FuncsData {
					limits[strNodeID][funcName] = funcData.LimitIn
				}
			} else {
				limits[strNodeID] = map[string]float64{}
				for funcName := range strategy.funcs {
					limits[strNodeID][funcName] = 0
				}
			}
		}
		return nil
	})

	msg := MsgNodeInfoRecalc{
		MsgType:     StrMsgNodeInfoTypeRecalc,
		HAProxyHost: _config.NodeIP,
		HAProxyPort: constants.HAProxyPort,
		FuncLimits:  limits,
	}
	debugMsgNodeInfoRecalc(msg)

	err = communication.MarshAndPublish(msg)
	if err != nil {
		return err
	}
	return nil
}

func (strategy *RecalcStrategy) recalcStep2() error {
	// Calculate weights for functions in overloaded mode.
	for funcName, overloaded := range strategy.overloads {
		if !overloaded {
			continue
		}

		// Calculate the weights for this function.
		strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
			totLimitsOut := float64(0)

			// Loop on all node in _nodestbl, check if that node
			// has this function running; if is present sum the amount of
			// req/sec forwardable to this node.
			for _, entry := range entries {
				funcData, present := entry.FuncsData[funcName]
				if present {
					totLimitsOut += funcData.LimitOut
				}
			}

			if totLimitsOut <= 0 {
				// If no node is available to help me with this function, i
				// set totLimitsOut to 1, only to avoid division by zero
				// problems. All the weights will be zero anyway
				totLimitsOut = 1
			}

			// Loop on all all node in _nodestbl, if function funcName is present in this node
			// that is in "oveload" state, is present also in i-th node, calculate
			// weight for the instance of function in i-th node.
			// Weight is based on LimitOut (number of req/sec forwardable to this node)
			// divided by total forwardable request.
			// All multiplied by 100, that is the sum of weights; this op allow to
			// express weights as the percentage of requests forwarded by this node to
			// other functions that runs on other nodes.
			for _, entry := range entries {
				funcData, present := entry.FuncsData[funcName]
				if present {
					funcData.NodeWeight = uint(funcData.LimitOut * constants.HAProxyMaxWeight / totLimitsOut)
				}
			}

			return nil
		})
	}

	// Print content of nodestbl.
	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
		debugNodesTblContent(entries)
		return nil
	})

	// Update HAProxy config.
	strMyself := _p2pHost.ID().String()

	var hacfg *HACfgRecalc
	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
		hacfg = strategy.createHACfgObject(
			strMyself,
			_config.OpenFaaSHost,
			_config.OpenFaaSPort,
			_config.RecalcPeriod,
			entries,
			strategy.funcs,
		)
		return nil
	})

	if err := strategy.updateHAProxyConfig(hacfg); err != nil {
		return fmt.Errorf("Updating HAProxy config: %w", err)
	}

	return nil
}

// processMsgNodeInfoRecalc processes a node info message received from pubsub.
func (strategy *RecalcStrategy) processMsgNodeInfoRecalc(sender string, msg *MsgNodeInfoRecalc) error {
	logger := logging.Logger()
	myself := _p2pHost.ID().String()

	if sender == myself {
		return nil // Ignore ourselves
	}

	if logging.GetDebugMode() {
		var debugBuffer strings.Builder
		debugBuffer.WriteString(fmt.Sprintf("Received node info message from node %s\n", sender))
		for _nodeID, _limits := range msg.FuncLimits {
			debugBuffer.WriteString(fmt.Sprintf("Functions limits for node %s:\n", _nodeID))
			for funcName := range _limits {
				debugBuffer.WriteString(fmt.Sprintf("\tFunction %s LimitOut: %f\n", funcName, _limits[funcName]))
			}
		}
		logger.Debug(debugBuffer.String())
	}

	// Note: if the sender node do not "know" us (we aren't in his FuncLimits)
	// we just ignore his message.
	funcLimits, present := msg.FuncLimits[myself]
	if !present {
		return nil
	}
	err := strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
		var debugBuffer strings.Builder
		debugBuffer.WriteString(fmt.Sprintf("Setting received values for node %s into table\n", sender))

		// If the message arrives from a sender node with ID nodeID that is not
		// present in _nodesbl yet, it is added to the table.
		_, present := entries[sender]
		if !present {
			entries[sender] = &nodestbl.EntryRecalc{
				FuncsData: map[string]*nodestbl.FuncData{},
			}
			debugBuffer.WriteString(fmt.Sprintf("Node %s was not present and has been added to the table\n", sender))
		}

		entries[sender].TAlive = time.Now()

		entries[sender].HAProxyHost = msg.HAProxyHost
		entries[sender].HAProxyPort = msg.HAProxyPort

		// Remove from my table the functions limits which are no more present
		// in the new updated message.
		debugBuffer.WriteString(fmt.Sprintf("Removing functions limits no more present in the received message from node %s\n", sender))
		for funcName := range entries[sender].FuncsData {
			_, present := funcLimits[funcName]
			if !present {
				delete(entries[sender].FuncsData, funcName)
				debugBuffer.WriteString(fmt.Sprintf("%s function is no more present in the received message from node %s and has been removed\n", funcName, sender))
			}
		}

		// Update the functions limits with the received values (also add new
		// functions which weren't present before).
		debugBuffer.WriteString(fmt.Sprintf("Updating functions limits with received values from node %s\n", sender))
		for funcName, limit := range funcLimits {
			_, present := entries[sender].FuncsData[funcName]
			if present {
				entries[sender].FuncsData[funcName].LimitOut = limit
				debugBuffer.WriteString(fmt.Sprintf("Updated LimitOut to %f for %s function of node %s\n", limit, funcName, sender))
			} else {
				entries[sender].FuncsData[funcName] = &nodestbl.FuncData{
					LimitIn:    0,
					LimitOut:   limit,
					NodeWeight: 0,
				}
				debugBuffer.WriteString(fmt.Sprintf("Set LimitOut to %f, LimitIn to 0 and NodeWeight to 0 for %s function of node %s\n", limit, funcName, sender))
			}
		}

		logger.Debug(debugBuffer.String())

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// updateHAProxyConfig updates the HAProxy config file with the provided object.
// Note: Go templates sort maps by key (see https://stackoverflow.com/questions/33860961/sorted-map-iteration-in-go-templates#comment76560806_33862260)
func (strategy *RecalcStrategy) updateHAProxyConfig(hacfg *HACfgRecalc) error {
	hacfg.Now = time.Now()

	// hacfg is a struct of type HACfgRecalc, that is use as content
	// for writing template file.
	// Indeed, in template file is possible to access fields
	// of this struct (see file of template .tmpl).
	// Template is referred to a data structure, in that case HACfgRecalc type.
	// See https://golang.org/pkg/text/template/.
	return strategy.hacfgupdater.UpdateHAConfig(hacfg)
}

// Method which creates and returns the HACfgRecalc object,
// used from method updateHAProxyConfig to update the HAProxy configuration
func (strategy *RecalcStrategy) createHACfgObject(
	myNodeID string,
	openFaaSHost string,
	openFaaSPort uint,
	recalcPeriod time.Duration,
	entries map[string]*nodestbl.EntryRecalc,
	funcLimits map[string]uint,
) *HACfgRecalc {
	hacfg := &HACfgRecalc{
		HACfg: HACfg{
			MyNodeID:     myNodeID,
			NodeIP:       _config.NodeIP,
			HAProxyHost:  _config.HAProxyHost,
			OpenFaaSHost: openFaaSHost,
			OpenFaaSPort: openFaaSPort,
		},

		StrRecalc:  recalcPeriod.String(),
		SecsRecalc: uint(recalcPeriod / time.Second),

		Nodes:     map[string]*HACfgNodeRecalc{},
		Functions: map[string]*HACfgFuncRecalc{},
	}

	// For each function in this node set limit (maxrate).
	for funcName, limit := range funcLimits {
		hacfg.Functions[funcName] = &HACfgFuncRecalc{
			Limit:    limit,
			Weights:  map[string]uint{},
			LimitsIn: map[string]uint{},
		}
	}

	// For each node write Host and port.
	for nodeID, entry := range entries {
		hacfg.Nodes[nodeID] = &HACfgNodeRecalc{
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
				hacfg.Functions[funcName] = &HACfgFuncRecalc{
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
