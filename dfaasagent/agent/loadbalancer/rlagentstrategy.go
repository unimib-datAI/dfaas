// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/httpserver"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/hasock"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/promq"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

type RLAgentStrategy struct {
	hacfgupdater  *hacfgupd.Updater
	nodestbl      *nodestbl.TableNMS
	offuncsClient *offuncs.Client
	promq         *promq.Client
	httpClient    *http.Client

	funcs               []string // Our OpenFaaS functions.
	commonNeighboursNum int      // Number of neighbours with at least a function in common.

	// Map of target nodes, with node ID of a common neighbour as key,
	// and array of common functions as value
	targetNodes map[string][]string

	// For each function deployed at node we have a map containing for each node
	// the corresponding weight. The node can be a remote one, local one or
	// "reject", a special node for direct rejection. The weights must sums to
	// 100 (they're percentuals).
	weights map[string]map[string]uint

	// The RL Agent strategy consists of two cycling phases.
	// allLocalPhaseTimestamp and rlAgentPhaseTimestamp store the timestamps
	// indicating when each phase starts. They may be zero if not yet set,
	// typically at the beginning of the strategy.
	allLocalPhaseTimestamp time.Time
	rlAgentPhaseTimestamp  time.Time
}

// strategyPhase represents the two possibile phases of the RL Agent strategy.
type strategyPhase int

const (
	allLocalPhase strategyPhase = iota
	rlAgentPhase
)

func (p strategyPhase) String() string {
	switch p {
	case allLocalPhase:
		return "allLocalPhase"
	case rlAgentPhase:
		return "rlAgentPhase"
	default:
		return "Unknown"
	}
}

// strategyPhaseFromStage returns the current RL Agent strategy phase based on
// the given k6 stage index.
func strategyPhaseFromStage(stage int) (strategyPhase, error) {
	if stage < 0 {
		return allLocalPhase, errors.New("found negative stage, expected positive or zero")
	}

	switch stage % 4 {
	case 0, 1:
		return allLocalPhase, nil
	case 2, 3:
		return rlAgentPhase, nil
	default:
		return allLocalPhase, nil // Unreachable.
	}
}

// RunStrategy handles the execution of the strategy. It is ran in a goroutine.
func (strategy *RLAgentStrategy) RunStrategy() error {
	logger := logging.Logger()
	logger.Debug("Starting RL Agent strategy...")

	logger.Info("Initial set-up: updating proxy with the discovered functions")
	if err := strategy.setup(); err != nil {
		return fmt.Errorf("failed to do initial set-up: %w", err)
	}
	// We start with a nil phase, since we do not detected any stage.
	var previousPhase *strategyPhase

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		start := time.Now()

		// Get the "General Purpose Table 0" (gpt0) field for the key "global"
		// in the stick-table "main" (a frontend) from HAProxy. This field
		// represents the current "Stage" of k6, which HAProxy stores by parsing
		// the "DFaaS-K6-Stage" header from incoming requests.
		//
		// Note the value is always added with +1, so if it is 0 means no header
		// has been found.
		stage, err := hasock.StickTableField(strategy.httpClient, "main", "gpt0")
		if err != nil {
			if errors.Is(err, hasock.ErrEmpty) {
				logger.Warn("Cannot detect current stage: empty gpt0 field for stick-table main. Skipping iteration")
				continue
			}
			return fmt.Errorf("reading gpt0 field in stick-table main from HAProxy: %w", err)
		}
		if stage == 0 {
			logger.Warn("Cannot detect current stage: requests do not have DFaaS-K6-Stage header. Skipping iteration")
			continue
		} else {
			// We must subtract 1 because the HAProxy config always add +1 to
			// the stage counter to be able to represent the value 0 as "no
			// stage header found".
			stage--
		}
		logger.Infof("Current detected stage: %d", stage)

		// From the current stage, we determine the strategy phase. The RL agent
		// operates in two phases:
		//
		//  1) In the first phase, the DFaaS node processes all incoming
		//  requests locally.
		//
		//  2) In the second phase, the DFaaS node uses statistics collected
		//  from both the first phase and previous second phase to query the RL
		//  model for an action, then applies that action (which may involve
		//  rejecting requests or forwarding them to other nodes).
		//
		// Since k6 is used for load testing, each stage consists of two
		// internal substages: stages 0 and 1 correspond to the "all-local"
		// phase, while stages 2 and 3 correspond to the RL-driven phase. This
		// pattern then repeats. Therefore, we can determine the current phase
		// by applying a modulo operation on the stage index.
		currentPhase, err := strategyPhaseFromStage(stage)
		if err != nil {
			logger.Warnf("Failed converting detected stage to strategy phase, skipping iteration: %w", err)
			continue
		}

		if previousPhase == nil {
			if currentPhase == allLocalPhase {
				logger.Infof("Starting strategy in %q phase", currentPhase)
			} else {
				logger.Warn("Previous phase is nil, current is not allLocalPhase. Skipping iteration")
				continue
			}
		} else if *previousPhase == currentPhase {
			logger.Infof("Current phase is equal to previous (%s). Nothing to do.", currentPhase)
			continue
		} else {
			logger.Infof("Moving from %q to %q phase", *previousPhase, currentPhase)
		}

		switch currentPhase {
		case allLocalPhase:
			err = strategy.allLocalPhase()
		case rlAgentPhase:
			err = strategy.rlAgentPhase()
		default:
			return fmt.Errorf("invalid phase %q", currentPhase)
		}
		if err != nil {
			return fmt.Errorf("running phase %q: %w", currentPhase, err)
		}

		previousPhase = &currentPhase

		duration := time.Since(start)
		httpserver.StrategyIterationDuration.Set(duration.Seconds())
		logger.Infof("Iteration completed. Duration: %s", duration.String())
	}

	return nil
}

// OnReceived is executed every time a message from a peer is received.
func (strategy *RLAgentStrategy) OnReceived(msg *pubsub.Message) error {
	// Check the type before deserializing to the right object.
	/*
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
	*/
	return nil
}

/*
func (strategy *RLAgentStrategy) calculateWeights() (map[string]map[string]uint, error) {
    weights := make(map[string]map[string]uint)

    // For each function (for now single function):
    //  1. Get data from previous obs:
    //      - Previous input rate
    //      -
    //  2. Get new local data:
    //      - Input rate
    //      - Reject Rate
    //      - AvgRespTimeLoc
    //      - CPUUtilization
    //      - NReplicas
    //  3. Get neighbor data:
    //      - Previous rejected fwd to neighbors (rate)
    //      - Previous fwd to neighbors (rate)
    //      -
    //  3. Build observation
    //  4. Call RL Agent to get action
    //  5. Parse action and convert to weights.

    weights := map[string]map[string]uint{
		"funcA": {"node1": 40, "node2": 40, "reject": 20},
		"funcB": {"node1": 50, "node2": 50},
		"funcC": {"node1": 60, "node2": 30, "reject": 5}, // invalid
	}

    if !weightsArePercentual(weights) {
        return nil, fmt.Errorf("weights sum is not 100: %v", weights)
    }
}

// weightsArePercentual returns true if the given weights for each function are
// percentuals (the sums equals 100), otherwise returns false.
func weightsArePercentual(weights map[string]map[string]uint) bool {
	for _, nodeWeights := range weights {
		var total uint = 0

		for _, w := range nodeWeights {
			total += w
		}

		if total != 100 {
            return false
		}
	}
	return true
}

func prettyPrintWeights(weights map[string]map[string]uint) {
	// Sort function names.
	funcNames := make([]string, 0, len(weights))
	for f := range weights {
		funcNames = append(funcNames, f)
	}
	sort.Strings(funcNames)

	for _, f := range funcNames {
		fmt.Printf("Function: %s\n", f)

		nodes := weights[f]

		// Separate "reject" from other nodes
		var rejectWeight uint
		keys := make([]string, 0, len(nodes))
		for k := range nodes {
			if k == "reject" {
				rejectWeight = nodes[k]
			} else {
				keys = append(keys, k)
			}
		}

		sort.Strings(keys) // sort other nodes

		// Print normal nodes
		for _, k := range keys {
			fmt.Printf("  %s -> %d\n", k, nodes[k])
		}

		// Print reject last if present
		if _, ok := nodes["reject"]; ok {
			fmt.Printf("  reject -> %d\n", rejectWeight)
		}
	}
}

func (strategy *RLAgentStrategy) calculateWeightsOld() (map[string]map[string]uint, error) {
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
func (strategy *RLAgentStrategy) publishNodeInfo() error {
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
func (strategy *RLAgentStrategy) updateCommonNeighbours() {
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
func (strategy *RLAgentStrategy) setProxyWeights() error {
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
func (strategy *RLAgentStrategy) createHACfgObject(
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
func (strategy *RLAgentStrategy) processMsgNodeInfoStatic(sender string, msg *MsgNodeInfoStatic) error {
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
*/

// setup() runs the initial setup of the strategy, mainly configuring HAProxy to
// process all incoming requests locally.
func (strategy *RLAgentStrategy) setup() error {
	if err := strategy.setAllLocal(); err != nil {
		return fmt.Errorf("failed to configure proxy: %w", err)
	}

	// We also set-up the internal http.Client used to contact the Data Plane
	// API when asking for the current stage. This allows used to reduce
	// connection overhead by reusing the same HTTP client.
	strategy.httpClient = &http.Client{
		Timeout: 900 * time.Millisecond,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return nil
}

// allLocalPhase runs the phase where the DFaaS agent processes all incoming
// requests locally.
func (strategy *RLAgentStrategy) allLocalPhase() error {
	if err := strategy.setAllLocal(); err != nil {
		return fmt.Errorf("failed to configure proxy: %w", err)
	}

	strategy.allLocalPhaseTimestamp = time.Now()

	return nil
}

// rlAgentPhase runs the phase where the DFaaS agent builds the observation for
// the RL model, queries the model, and applies the resulting action to the
// proxy.
func (strategy *RLAgentStrategy) rlAgentPhase() error {
	// Build observation
	strategy.buildObservation()

	// Ask RL model

	// Apply action

	return nil
}

func (strategy *RLAgentStrategy) setAllLocal() error {
	// Most of the code is taken from alllocal.go
	funcs, err := strategy.offuncsClient.GetFuncsWithTimeout()
	if err != nil {
		return fmt.Errorf("get function metadata: %w", err)
	}

	// Add 1 seconds to base timeout (if given) to all functions.
	for _, timeout := range funcs {
		if timeout != nil {
			*timeout += 1000
		}
	}

	debugFuncs(funcs)
	if err = strategy.updateProxyConfiguration(funcs); err != nil {
		return fmt.Errorf("updating proxy config: %w", err)
	}
	return nil
}

// updateProxyConfiguration updates the HAProxy configuration with the provided
// list of deployed functions. HAProxy will always be reloaded after the update.
func (strategy *RLAgentStrategy) updateProxyConfiguration(funcs map[string]*uint) error {
	// Define and populate this anonymous struct to pass data to the Go
	// template.
	data := struct {
		Now          string
		DFaaSNodeID  string
		Functions    map[string]*uint
		OpenFaaSHost string
		OpenFaaSPort uint
	}{
		Now:          time.Now().Format("2006-01-02 15:04:05"),
		DFaaSNodeID:  _p2pHost.ID().String(),
		Functions:    funcs,
		OpenFaaSHost: _config.OpenFaaSHost,
		OpenFaaSPort: _config.OpenFaaSPort,
	}

	return strategy.hacfgupdater.UpdateHAConfig(data)
}

func (strategy *RLAgentStrategy) buildObservation() ([]byte, error) {
	if strategy.allLocalPhaseTimestamp.IsZero() {
		return nil, errors.New("allLocalPhaseTimestamp is not set, but is required for rlAgentPhase")
	}
	now := time.Now()
	if now.Before(strategy.allLocalPhaseTimestamp) {
		return nil, errors.New("allLocalPhaseTimestamp cannot be greater than time.Now()!")
	}

	obs := make(map[string]any)

	// input_rate key in observation.
	inputRate, err := strategy.promq.InputRate(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'input_rate' key: %w", err)
	}
	// Because we currently support only one function!
	inputRateSingle, err := extractSingleFunctionValue(inputRate)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'input_rate' key: %w", err)
	}
	obs["input_rate"] = inputRateSingle

	// previous_input_rate key in observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_input_rate"] = 0
	} else {
		inputRate, err := strategy.promq.InputRate(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_input_rate' key: %w", err)
		}
		inputRateSingle, err := extractSingleFunctionValue(inputRate)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_input_rate' key: %w", err)
		}
		obs["previous_input_rate"] = inputRateSingle
	}

	// previous_fwd_to_node_X key in observation.
	peers := 0
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		for _, peer := range _p2pHost.Network().Peers() {
			key := fmt.Sprintf("previous_fwd_to_node_%s", peer.String())
			obs[key] = 0
			peers++
		}
	} else {
		prevForwardRate, err := strategy.promq.ForwardRate(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: %w", err)
		}
		prevForwardRateSingle, err := extractSingleFunctionValue(prevForwardRate)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: %w", err)
		}
		for peer, rate := range prevForwardRateSingle {
			// ForwardRate() always returns openfaas-local node that is the
			// local one (not remote).
			if peer == "openfaas-local" {
				continue
			}
			key := fmt.Sprintf("previous_fwd_to_%s", peer)
			obs[key] = rate
		}
	}
	// FIXME: Remove this code (used for debugging).
	if peers != 4 {
		return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: found %d peers, expected 4: peers", peers)
	}

	// FIXME: Enable dynamic neighbours. Currently not supported.
	obs["previous_fwd_to_node_1_rejected"] = 0
	obs["previous_fwd_to_node_2_rejected"] = 0
	obs["previous_fwd_to_node_3_rejected"] = 0
	obs["previous_fwd_to_node_4_rejected"] = 0

	// reject_rate key in observation.
	rejectRate, err := strategy.promq.RejectRate(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'reject_rate' key: %w", err)
	}
	rejectRateSingle, err := extractSingleFunctionValue(rejectRate)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'reject_rate' key: %w", err)
	}
	obs["rejectRateSingle"] = rejectRateSingle

	// previous_reject_rate key in observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_reject_rate"] = 0.0
	} else {
		prevRejectRate, err := strategy.promq.RejectRate(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_reject_rate' key: %w", err)
		}
		prevRejectRateSingle, err := extractSingleFunctionValue(prevRejectRate)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_reject_rate' key: %w", err)
		}
		obs["previous_reject_rate"] = prevRejectRateSingle
	}

	// avg_resp_time_loc key in observation.
	avgRespTime, err := strategy.promq.AvgRespTimeLocal(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'avg_resp_time_loc' key: %w", err)
	}
	avgRespTimeSingle, err := extractSingleFunctionValue(avgRespTime)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'avg_resp_time_loc' key: %w", err)
	}
	obs["avg_resp_time_loc"] = avgRespTimeSingle

	// previous_avg_resp_time_loc key in observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_avg_resp_time_loc"] = 0.0
	} else {
		prevAvgRespTime, err := strategy.promq.AvgRespTimeLocal(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_loc' key: %w", err)
		}
		prevAvgRespTimeSingle, err := extractSingleFunctionValue(prevAvgRespTime)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_loc' key: %w", err)
		}
		obs["previous_avg_resp_time_loc"] = prevAvgRespTimeSingle
	}

	// FIXME: Enable dynamic neighbours. Currently not supported.
	obs["previous_avg_resp_time_fwd_to_node_1"] = 0.0
	obs["previous_avg_resp_time_fwd_to_node_2"] = 0.0
	obs["previous_avg_resp_time_fwd_to_node_3"] = 0.0
	obs["previous_avg_resp_time_fwd_to_node_4"] = 0.0

	// cpu_utilization key in observation.
	// FIXME: Only one function is supported!
	cpuUsage, err := strategy.promq.CPUUsage(strategy.allLocalPhaseTimestamp,
		now,
		[]string{"figlet"})
	if err != nil {
		return nil, fmt.Errorf("building observation for 'cpu_utilization' key: %w", err)
	}
	cpuUsageSingle, err := extractSingleFunctionValue(cpuUsage)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'cpu_utilization' key: %w", err)
	}
	obs["cpu_utilization"] = cpuUsageSingle

	// previous_cpu_utilization key in observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_cpu_utilization"] = 0.0
	} else {
		// FIXME: Only one function is supported!
		prevCPUUsage, err := strategy.promq.CPUUsage(strategy.rlAgentPhaseTimestamp,
			strategy.allLocalPhaseTimestamp,
			[]string{"figlet"})
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_cpu_utilization' key: %w", err)
		}
		prevCPUUsageSingle, err := extractSingleFunctionValue(prevCPUUsage)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_cpu_utilization' key: %w", err)
		}
		obs["previous_cpu_utilization"] = prevCPUUsageSingle
	}

	// n_replicas key observation.
	replicas, err := strategy.promq.Replicas(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'n_replicas' key: %w", err)
	}
	replicasSingle, err := extractSingleFunctionValue(replicas)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'n_replicas' key: %w", err)
	}
	obs["n_replicas"] = replicasSingle

	// previous_n_replicas key observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_n_replicas"] = 1
	} else {
		prevReplicas, err := strategy.promq.Replicas(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_n_replicas' key: %w", err)
		}
		prevReplicasSingle, err := extractSingleFunctionValue(prevReplicas)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_n_replicas' key: %w", err)
		}
		obs["previous_n_replicas"] = prevReplicasSingle
	}

	node_id := fmt.Sprintf("node_%s", _p2pHost.ID().String())
	payload := map[string]any{
		"observation": map[string]any{node_id: obs},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to build JSON observation: %w", err)
	}
	return data, nil
}

// FIXME: Remove this! Use because currently we support observations for the RL
// model with only a single function.
func extractSingleFunctionValue[T any](funcs map[string]T) (T, error) {
	// FIXME: Make configurable!
	name := "figlet"

	value, exists := funcs[name]
	if !exists {
		var zero T
		return zero, fmt.Errorf("missing function %q in %v", name, funcs)
	}
	return value, nil
}
