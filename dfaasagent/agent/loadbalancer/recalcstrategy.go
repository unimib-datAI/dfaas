// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bcicen/go-haproxy"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"

	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/communication"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/constants"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/hacfgupd"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/hasock"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/offuncs"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/ofpromq"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/utils/p2phostutils"
)

// In this file is implemented the Recalc strategy

// Struct representing a RecalcStrategy instance, which implements the Strategy interface
type RecalcStrategy struct {
	hacfgupdater  hacfgupd.Updater
	nodestbl      *nodestbl.TableRecalc
	offuncsClient *offuncs.Client
	recalc        recalc
	it            int // = 0 // Number of agent loop iterations
}

// Private struct containing variables specific to the recalc algorithm, which
// need to be shared amongst the two recalc steps
type recalc struct {
	nodeIDs         []peer.ID                     // IDs of the connected p2p nodes
	stats           []*haproxy.Stat               // HAProxy stats
	funcs           map[string]uint               // Our OpenFaaS functions with dfaas.maxrate limits
	userRates       map[string]float64            // Invocation rates for users only (in req/s) (from HAProxy stick-tables)
	afet            map[string]float64            // Average Function Execution Times (from Prometheus)
	invoc           map[string]map[string]float64 // Invocation rates (in req/s) (from Prometheus)
	serviceCount    map[string]int
	cpuUsage        map[string]float64
	ramUsage        map[string]float64
	perFuncCpuUsage map[string]float64
	perFuncRamUsage map[string]float64

	// For each function, the value is true if the node is currently in overload
	// mode (req/s >= maxrate), false if underload
	overloads map[string]bool
}

//////////////////// PUBLIC FUNCTIONS FOR RECALC ////////////////////

// RunStrategy handles the periodic execution of the recalculation function. It
// should run in a goroutine
func (strategy *RecalcStrategy) RunStrategy() error {
	// Obtain the global logger object
	logger := logging.Logger()

	var millisNow, millisSleep int64
	var err error

	if _config.RecalcPeriod == 0 {
		logger.Warn("Given RecalcPeriod must be a positive time duration, using 1 minute by default")
		_config.RecalcPeriod = 1 * time.Minute
	}

	millisInterval := int64(_config.RecalcPeriod / time.Millisecond)
	millisIntervalHalf := millisInterval / 2

	for {
		millisNow = time.Now().UnixNano() / 1000000
		millisSleep = millisInterval - (millisNow % millisInterval)
		time.Sleep(time.Duration(millisSleep) * time.Millisecond)

		if err := strategy.recalcStep1(); err != nil {
			logger.Error("Failed Recalc step 1, skipping RunStrategy iteration ", err)
			logger.Warn("Waiting 5 second before retrying RunStrategy")
			time.Sleep(5 * time.Second)
			continue
		}

		millisNow = time.Now().UnixNano() / 1000000
		millisSleep = millisInterval - ((millisNow + millisIntervalHalf) % millisInterval)
		time.Sleep(time.Duration(millisSleep) * time.Millisecond)

		err = strategy.recalcStep2()
		if err != nil {
			return err
		}
	}
}

// OnReceived should be executed every time a message from a peer is received
func (strategy *RecalcStrategy) OnReceived(msg *pubsub.Message) error {
	var msgForType struct{ MsgType string }
	err := json.Unmarshal(msg.GetData(), &msgForType)
	if err != nil {
		return errors.Wrap(err, "Error while deserializing a message from the PubSub subscription")
	}

	switch msgForType.MsgType {
	case StrMsgTextType:
		var objMsg MsgText
		err := json.Unmarshal(msg.GetData(), &objMsg)
		if err != nil {
			return errors.Wrap(err, "Error while deserializing a message from the PubSub subscription")
		}

		processMsgText(msg.GetFrom().String(), &objMsg)
	case StrMsgNodeInfoTypeRecalc:
		var objMsg MsgNodeInfoRecalc
		err := json.Unmarshal(msg.GetData(), &objMsg)
		if err != nil {
			return errors.Wrap(err, "Error while deserializing a message from the PubSub subscription")
		}

		strategy.processMsgNodeInfoRecalc(msg.GetFrom().String(), &objMsg)
	}

	return nil
}

//////////////////// PRIVATE FUNCTIONS FOR RECALC ////////////////////

func (strategy *RecalcStrategy) recalcStep1() error {
	var err error
	logger := logging.Logger()
	millisNow := time.Now().UnixNano() / 1000000
	logger.Debugf("#################### RECALC: STEP 1 (UnixMillis %d) ####################", millisNow)

	//////////////////// EXAMPLE TEXT MESSAGE ////////////////////

	//err := communication.MarshAndPublish(MsgText{
	//	MsgType: StrMsgTextType,
	//	Text:    "I'm alive!",
	//})
	//if err != nil {
	//	return err
	//}

	//////////////////// GET LIST OF CONNECTED NODES ////////////////////

	strategy.recalc.nodeIDs = p2phostutils.GetConnNodeIDsUniq(_p2pHost)
	debugConnectedNodes(strategy.recalc.nodeIDs)

	//////////////////// GATHER HAPROXY STATS ////////////////////

	//_recalc.stats, err = _hasockClient.Stats()
	//if err != nil {
	//	return errors.Wrap(err, "Error while gathering HAProxy stats from socket")
	//}
	//debugHAProxyStats(_recalc.stats)

	//////////////////// GATHER INFO ABOUT OPENFAAS FUNCTIONS ////////////////////

	strategy.recalc.funcs, err = strategy.offuncsClient.GetFuncsWithMaxRates()
	if err != nil {
		return errors.Wrap(err, "get functions info from OpenFaaS")
	}
	debugFuncs(strategy.recalc.funcs)

	//////////////////// GATHER INFO FROM HAPROXY STICKTABLES st_users_func_* ////////////////////

	strategy.recalc.userRates = map[string]float64{}

	for funcName := range strategy.recalc.funcs {
		stName := fmt.Sprintf("st_users_func_%s", funcName)
		stContent, err := hasock.ReadStickTable(stName)

		if err != nil {
			errWrap := errors.Wrap(err, "Error while reading the stick-table \""+stName+"\" from the HAProxy socket")
			logger.Error(errWrap)
			logger.Warn("Not changing userRates for stick-table \"" + stName + "\" but this should be ok")
		} else {
			for _, stEntry := range stContent {
				// There should be only one line, with key "80", which is the port of the HAProxy frontend
				strategy.recalc.userRates[funcName] = float64(stEntry.HTTPReqCnt) / float64(_config.RecalcPeriod/time.Second) * 2
				// Note: the whole formula is multiplied by two at the end because we know we restarted HAProxy at the end of recalcStep2
			}
		}

		debugStickTable(stName, stContent)
	}
	debugHAProxyUserRates(strategy.recalc.userRates)

	//////////////////// [NEW] GATHER INFO FROM HAPROXY STICKTABLES st_local_func_* ////////////////////

	for funcName := range strategy.recalc.funcs {
		stName := fmt.Sprintf("st_local_func_%s", funcName)
		stContent, err := hasock.ReadStickTable(stName)

		if err != nil {
			errWrap := errors.Wrap(err, "Error while reading the stick-table \""+stName+"\" from the HAProxy socket")
			logger.Error(errWrap)
			logger.Warn("Not changing local rates for stick-table \"" + stName + "\" but this should be ok")
		}

		debugStickTable(stName, stContent)
	}

	//////////////////// [NEW] GATHER INFO FOR STICKTABLES OF DATA FROM OTHER NODES ////////////////////
	/*
		for funcName := range _recalc.funcs {
			for _, nodeID := range _recalc.nodeIDs {
				stName := fmt.Sprintf("st_other_node_%s_%s", funcName, nodeID.String())
				stContent, err := hasock.ReadStickTable(&_hasockClient, stName)

				if err != nil {
					errWrap := errors.Wrap(err, "Error while reading the stick-table \""+stName+"\" from the HAProxy socket")
					logger.Error(errWrap)
					logger.Warn("Not changing other nodes rates for stick-table \"" + stName + "\" but this should be ok")
				}

				debugStickTable(stName, stContent)
			}
		}
	*/
	//////////////////// GATHER INFO FROM PROMETHEUS ////////////////////

	strategy.recalc.afet, err = ofpromq.QueryAFET(_config.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while execting Prometheus query")
	}
	debugPromAFET(_config.RecalcPeriod, strategy.recalc.afet)

	strategy.recalc.invoc, err = ofpromq.QueryInvoc(_config.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromInvoc(_config.RecalcPeriod, strategy.recalc.invoc)

	strategy.recalc.serviceCount, err = ofpromq.QueryServiceCount()
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromServiceCount(strategy.recalc.serviceCount)

	strategy.recalc.cpuUsage, err = ofpromq.QueryCPUusage(_config.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromCPUusage(_config.RecalcPeriod, strategy.recalc.cpuUsage)

	strategy.recalc.ramUsage, err = ofpromq.QueryRAMusage(_config.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromRAMusage(_config.RecalcPeriod, strategy.recalc.ramUsage)

	if len(strategy.recalc.funcs) > 0 {
		// Get function's name as a slice.
		funcNames := make([]string, len(strategy.recalc.funcs))
		i := 0
		for k := range strategy.recalc.funcs {
			funcNames[i] = k
			i++
		}

		strategy.recalc.perFuncCpuUsage, err = ofpromq.QueryCPUusagePerFunction(_config.RecalcPeriod, funcNames)
		if err != nil {
			return errors.Wrap(err, "Error while executing Prometheus query")
		}
		debugPromCPUusagePerFunction(_config.RecalcPeriod, strategy.recalc.perFuncCpuUsage)

		strategy.recalc.perFuncRamUsage, err = ofpromq.QueryRAMusagePerFunction(_config.RecalcPeriod, funcNames)
		if err != nil {
			return errors.Wrap(err, "Error while executing Prometheus query")
		}
		debugPromRAMusagePerFunction(_config.RecalcPeriod, strategy.recalc.perFuncRamUsage)
	}

	//////////////////// OVERLOAD / UNDERLOAD MODE DECISION ////////////////////

	strategy.recalc.overloads = map[string]bool{}

	for funcName, maxRate := range strategy.recalc.funcs {
		logger.Debugf("Computing if %s function is on overload", funcName)
		invocRate, present := strategy.recalc.userRates[funcName]

		if !present || invocRate < float64(maxRate) {
			strategy.recalc.overloads[funcName] = false
		} else {
			strategy.recalc.overloads[funcName] = true
		}
	}
	debugOverloads(strategy.recalc.overloads) // Debug purpose.

	strategy.it++

	//////////////////// LIMITS AND WEIGHTS CALCULATIONS ////////////////////

	for funcName, ovrld := range strategy.recalc.overloads {
		logger.Debugf("Calculating limits and weights for %s function", funcName)
		if ovrld {
			// Set all funcData.LimitIn to zero for this function
			logger.Debugf("%s function is on overload! Setting LimitIn to 0", funcName)
			strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
				for _, entry := range entries {
					funcData, present := entry.FuncsData[funcName]
					if present {
						funcData.LimitIn = 0
					}
				}

				return nil
			})
		} else {
			// Calculate the rate margin
			logger.Debugf("Calculating rate margin for %s function", funcName)
			invocRate, present := strategy.recalc.userRates[funcName]
			maxRate := strategy.recalc.funcs[funcName]
			logger.Debugf("%s function invocation rate is %f", funcName, invocRate)
			logger.Debugf("%s function max rate is %d", funcName, maxRate)
			var margin uint
			if present {
				margin = maxRate - uint(invocRate)
			} else {
				margin = maxRate
			}

			logger.Debugf("%s function margin equal to %d", funcName, margin)

			// Set all funcData.Weight to zero for this function, and set the
			// LimitIn for each node
			logger.Debugf("Setting Weight to 0 for %s function and setting LimitIn for each node", funcName)
			strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
				nNodes := uint(0)

				for _, entry := range entries {
					funcData, present := entry.FuncsData[funcName]
					if present {
						// Weights represent likelihood of send a request toward i-th
						// function instance.
						// Considering that this function instance is labelled as "underload"
						// it is not necessary to send request towards other nodes.
						funcData.NodeWeight = 0
						nNodes++
						logger.Debugf("Set Weight to 0 for %s function", funcName)
					}
				}

				// Note: if nNodes == 0, it means that (for now) i am the only
				// one to have this function, so i don't have to set the LimitIn
				// for anyone because no one needs it. Note also that the
				// nodestbl.SetReceivedValues() function sets the LimitIn to
				// zero, so not setting it here is ok

				if nNodes > 0 {
					limitIn := margin / nNodes // Equal distribution! May be
					// replaced in the future with a more efficient algorithm

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
	}

	//////////////////// PRINT CONTENT OF NODESTBL ////////////////////

	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
		debugNodesTblContent(entries)
		return nil
	})

	//////////////////// P2P MESSAGES PUBLISHING ////////////////////

	limits := map[string]map[string]float64{}

	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
		for _, nodeID := range strategy.recalc.nodeIDs {
			strNodeID := nodeID.String()

			entry, present := entries[strNodeID]
			if present {
				// If this node has sent me some messages before, i send him the
				// limits according to the nodestbl
				limits[strNodeID] = map[string]float64{}
				for funcName, funcData := range entry.FuncsData {
					limits[strNodeID][funcName] = funcData.LimitIn
				}
			} else {
				// If this node has not sent me anything before, i send him all
				// the functions i have, but with all limits set to zero
				limits[strNodeID] = map[string]float64{}
				for funcName := range strategy.recalc.funcs {
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
	//////////////////// IF EVERYTHING OK, RETURN NIL ////////////////////

	return nil
}

func (strategy *RecalcStrategy) recalcStep2() error {
	var err error
	logger := logging.Logger()
	millisNow := time.Now().UnixNano() / 1000000
	logger.Debugf("#################### RECALC: STEP 2 (UnixMillis %d) ####################", millisNow)

	//////////////////// CALC WEIGHTS FOR FUNCTIONS IN OVERLOAD MODE ////////////////////

	for funcName, ovrld := range strategy.recalc.overloads {
		if ovrld {
			// Calculate the weights for this function
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
	}

	//////////////////// PRINT CONTENT OF NODESTBL ////////////////////

	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
		debugNodesTblContent(entries)
		return nil
	})

	//////////////////// UPDATE HAPROXY CONFIGURATION ////////////////////

	strMyself := _p2pHost.ID().String()

	var hacfg *HACfgRecalc
	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
		hacfg = strategy.createHACfgObject(
			strMyself,
			_config.OpenFaaSHost,
			_config.OpenFaaSPort,
			_config.RecalcPeriod,
			entries,
			strategy.recalc.funcs,
		)
		return nil
	})

	err = strategy.updateHAProxyConfig(hacfg)
	if err != nil {
		return err
	}

	//////////////////// IF EVERYTHING OK, RETURN NIL ////////////////////

	return nil
}

// processMsgNodeInfoRecalc processes a node info message received from pubsub
func (strategy *RecalcStrategy) processMsgNodeInfoRecalc(sender string, msg *MsgNodeInfoRecalc) error {
	logger := logging.Logger()
	myself := _p2pHost.ID().String()

	if sender == myself {
		return nil // Ignore ourselves
	}

	if logging.GetDebugMode() {
		logger.Debugf("Received node info message from node %s", sender)
		for _nodeID, _limits := range msg.FuncLimits {
			logger.Debugf("Functions limits for node %s:", _nodeID)
			for funcName := range _limits {
				logger.Debugf("	Function %s LimitOut: %f", funcName, _limits[funcName])
			}
		}
	}

	// Note: if the sender node do not "know" us (we aren't in his FuncLimits) we just ignore his message
	funcLimits, present := msg.FuncLimits[myself]
	if present {
		logger.Debugf("Setting received values for node %s into table", sender)
		return strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryRecalc) error {
			// If the message arrives from a sender node with ID nodeID that is
			// not present in _nodesbl yet, it is added to the table.
			_, present := entries[sender]
			if !present {
				entries[sender] = &nodestbl.EntryRecalc{
					FuncsData: map[string]*nodestbl.FuncData{},
				}
				logger.Debugf("Node %s was not present and has been added to the table", sender)
			}

			entries[sender].TAlive = time.Now()

			entries[sender].HAProxyHost = msg.HAProxyHost
			entries[sender].HAProxyPort = msg.HAProxyPort

			// Remove from my table the functions limits which are no more present
			// in the new updated message
			logger.Debugf("Removing functions limits no more present in the received message from node %s", sender)
			for funcName := range entries[sender].FuncsData {
				// Once this routine executed a message from another node of the
				// p2p net has been received.
				// If I (and I am the receiver node) stored in _nodestbl functions
				// that are not more present in sender node, identified by the fact
				// that they are not more present in received message, I can remove them
				// from my local table.
				_, present := funcLimits[funcName]
				if !present {
					delete(entries[sender].FuncsData, funcName)
					logger.Debugf("%s function is no more present in the received message from node %s and has been removed", funcName, sender)
				}
			}

			// Update the functions limits with the received values (also add new
			// functions which weren't present before)
			logger.Debugf("Updating functions limits with received values from node %s", sender)
			for funcName, limit := range funcLimits {
				_, present := entries[sender].FuncsData[funcName]
				if present {
					// For each function received by sender node, updates
					// corrisponding line of _nodestbl table.
					// If entry for sender node is present, updates LimitOut
					// for that node with received limit.
					// LimitOut means number of req/sec that I can fwd
					// toward this node.
					// Note: this LimitOut is updated on the base of LimitIn
					// for this function received by i-th node (sender).
					entries[sender].FuncsData[funcName].LimitOut = limit
					logger.Debugf("Updated LimitOut to %f for %s function of node %s", limit, funcName, sender)
				} else {
					entries[sender].FuncsData[funcName] = &nodestbl.FuncData{
						LimitIn:    0,
						LimitOut:   limit,
						NodeWeight: 0,
					}
					logger.Debugf("Set LimitOut to %f, LimitIn to 0 and NodeWeight to 0 for %s function of node %s", limit, funcName, sender)
				}
			}

			return nil
		})
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
