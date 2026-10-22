// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faas"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/promq"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/proxy"
)

// This file contains the implementation of the Recalc strategy.

// RecalcStrategy is the structure that implements the Strategy interface.
//
// Internally it works with 2 components:
//
// 1. A component that receives and processes messages coming from other nodes,
// updating the table of Limits. That is the "OnReceived" method.
//
// 2. A component that synchronizes the limit and weight updates with k6
// iterations. The component is launched in a dedicated goroutine by the
// "RunStrategy" method.
//
// See also the corresponding createStrategy method to understand how the struct
// fields are initialized.
type RecalcStrategy struct {
	hacfgupdater *hacfgupd.Updater
	funcsClient  *faas.Client
	runtimeapi   *proxy.RuntimeAPI
	promq        *promq.Client

	// limitsTableLock is a read/write mutex to protect LimitsTable.
	limitsTableLock sync.RWMutex

	// limitsTable is a nested structure consisting of two maps. The first is a
	// mapping from node IDs to function names, the second is a mapping from
	// function names to rate limit.
	//
	// The meaning of FuncLimits[nodeID][funcName] is: for the neighbor nodeID,
	// this local node can send to it at maximum the given number of requests
	// per second for the function called funcName.
	//
	// This information is updated on every incoming message from neighbors and
	// read to update the HAProxy weights.
	limitsTable map[string]map[string]float32
}

// RunStrategy initializes the proxy and runs experiments sequentially.
// It should run in a dedicated goroutine.
func (strategy *RecalcStrategy) RunStrategy() error {
	logger := logging.Logger()

	if err := strategy.reloadProxy(); err != nil {
		return fmt.Errorf("failed to initialize proxy: %w", err)
	}

	// Need to wait to avoid reading sporious proxy variables.
	time.Sleep(time.Second)

	for {
		// Start periodic broadcasts while no experiment is running.
		stop := make(chan struct{})
		go strategy.broadcastLimitsPeriodically(stop)

		// Wait until k6 starts an experiment.
		if _, err := strategy.waitForProxyVariable("k6_exp_started"); err != nil {
			close(stop)
			return fmt.Errorf("failed waiting for experiment start: %w", err)
		}

		// Stop periodic broadcasts during the experiment.
		close(stop)
		logger.Info("k6 experiment started")

		if err := strategy.reloadProxy(); err != nil {
			return fmt.Errorf("failed to reload proxy at experiment start: %w", err)
		}
		// Same as the other sleep.
		time.Sleep(time.Second)

		if err := strategy.RunExperiment(); err != nil {
			return fmt.Errorf("experiment failed: %w", err)
		}

		logger.Info("k6 experiment completed")
	}
}

// RunExperiment synchronizes limit and weight updates with k6 iterations.
// It returns when the current experiment ends.
func (strategy *RecalcStrategy) RunExperiment() error {
	logger := logging.Logger().With("strategy_component", "RunExperiment")

	var previousIteration int
	initialized := false
	for {
		// Check whether the experiment has ended.
		_, ended, err := strategy.runtimeapi.Variable("k6_exp_ended")
		if err != nil {
			return fmt.Errorf("failed to check experiment status: %w", err)
		}
		if ended {
			logger.Info("k6 experiment ended")
			return nil
		}

		// We expect k6 to attach the current iteration to every incoming
		// request.
		value, err := strategy.waitForProxyVariable("k6_iteration")
		if err != nil {
			return fmt.Errorf("failed waiting for k6 iteration: %w", err)
		}

		currentIteration, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid k6_iteration value %q: %w", value, err)
		}

		// Process each iteration only once.
		if initialized && currentIteration == previousIteration {
			time.Sleep(time.Second)
			continue
		}

		if !initialized {
			logger.Infow("First k6 iteration detected", "iteration", currentIteration)
			initialized = true
		} else {
			logger.Infow("New k6 iteration detected, resetting proxy weights",
				"previous_iteration", previousIteration,
				"current_iteration", currentIteration,
			)

			// At the start of each iteration, we reset the weights to their
			// initial configuration. Instead of reloading the proxy, we update
			// the weights directly.
			if err := strategy.setAllTrafficLocal(); err != nil {
				logger.Errorf("Failed to reset proxy weights: %v", err)
			}
		}

		previousIteration = currentIteration

		// Read the rate configured by k6 for this iteration.
		//
		// FIXME: This limits the strategy to a single function and does not
		// infer the input rate from proxy statistics.
		rate, found, err := strategy.runtimeapi.Variable("k6_iteration_rate")
		if err != nil {
			return fmt.Errorf("failed to read k6_iteration_rate: %w", err)
		}
		if !found {
			return fmt.Errorf("k6_iteration_rate variable not found")
		}

		parsedRate, err := strconv.ParseFloat(rate, 32)
		if err != nil {
			return fmt.Errorf("invalid k6_iteration_rate value %q: %w", rate, err)
		}
		if parsedRate < 0 || math.IsNaN(parsedRate) || math.IsInf(parsedRate, 0) {
			return fmt.Errorf("invalid k6_iteration_rate value %q", rate)
		}

		iterationRate := float32(parsedRate)
		logger.Debugw("Retrieved k6 iteration rate",
			"iteration", currentIteration,
			"rate", iterationRate,
		)

		// Publish the available limits to neighbors.
		if err := strategy.updateLimitsForNeighbors(iterationRate); err != nil {
			return fmt.Errorf("failed to update neighbor limits: %w", err)
		}

		// Allow neighbor updates to propagate.
		time.Sleep(4 * time.Second)

		// FIXME: We assume that the time between the message propagation and
		// the local weights update the k6_iteration is the same.

		// Apply the weights for the current iteration.
		if err := strategy.updateLocalWeights(); err != nil {
			return fmt.Errorf("failed to update local weights: %w", err)
		}
	}
}

// OnReceived is executed every time a message from a peer is received.
func (strategy *RecalcStrategy) OnReceived(msg *pubsub.Message) error {
	// We need first to check the message's type...
	var msgForType struct{ MsgType string }
	if err := json.Unmarshal(msg.GetData(), &msgForType); err != nil {
		return fmt.Errorf("error while deserializing a message: %w", err)
	}

	if msgForType.MsgType != MsgTypeRecalc {
		logging.Logger().Warnf("Skipping message, unrecognized type %q", msgForType.MsgType)
		return nil
	}

	// ...and only now we can parse it completely.
	var parsedMsg MsgRecalc
	if err := json.Unmarshal(msg.GetData(), &parsedMsg); err != nil {
		logging.Logger().Warnf("Skipping message, failed to parse as MsgNodeInfoRecalc type: %v", err)
		return fmt.Errorf("error while deserializing a message: %w", err)
	}

	if err := strategy.processMsgRecalc(msg.GetFrom().String(), &parsedMsg); err != nil {
		return fmt.Errorf("failed to process recalc message: %w", err)
	}

	return nil
}

// broadcastLimitsPeriodically periodically publishes the available limits to
// neighboring nodes while no k6 experiment is running. It stops when the stop
// channel is closed.
func (strategy *RecalcStrategy) broadcastLimitsPeriodically(stop <-chan struct{}) {
	logger := logging.Logger().With("strategy_component", "broadcastLimitsPeriodically")
	ticker := time.NewTicker(_config.RecalcPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return

		case <-ticker.C:
			proxyReqRate, err := strategy.runtimeapi.ReqRate()
			if err != nil {
				logger.Errorf("Failed to get proxy request rate: %v", err)
				continue
			}

			funcs, err := strategy.funcsClient.WithMaxRates()
			if err != nil {
				logger.Errorf("Failed to get deployed functions: %v", err)
				continue
			}

			// FIXME: Support more than 1 function.
			if len(funcs) > 1 {
				logger.Errorf("Expected only 1 function, found %d", len(funcs))
				continue
			}

			for funcName := range funcs {
				backend := "function_" + funcName
				servers, present := proxyReqRate[backend]

				var iterationRate float32
				if present {
					iterationRate = servers["BACKEND"]
				} else {
					logger.Warnw("Backend not found, skipping margin broadcasting", "backend", backend)
					continue
				}

				if err := strategy.updateLimitsForNeighbors(iterationRate); err != nil {
					logger.Errorw("Failed to update neighbor limits for %q: %v", funcName, err)
				}
			}
		}
	}
}

// updateLimitsForNeighbors calculates the available margin based on the current
// local rate and publishes the information to neighbors.
func (strategy *RecalcStrategy) updateLimitsForNeighbors(iterationRate float32) error {
	logger := logging.Logger().With("strategy_component", "updateLimitsForNeighbors")

	// Build neighbors info.
	neighborsID := []string{} // Node's ID (node_XXX).
	for _, peer := range _p2pHost.Network().Peers() {
		// "node_ID" is required format.
		peerID := fmt.Sprintf("node_%s", peer)
		neighborsID = append(neighborsID, peerID)
	}
	if len(neighborsID) == 0 {
		logger.Warn("No available neighbors, skipping message publication")
		return nil
	}
	logger.Debugw("Collected neighbors",
		"neighbors_count", len(neighborsID),
		"neighbors", neighborsID,
	)

	// Get deployed functions with associated limits.
	funcs, err := strategy.funcsClient.WithMaxRates()
	if err != nil {
		return fmt.Errorf("failed to get deployed functions: %w", err)
	}

	// For now, k6_iteration_rate refers to the only deployed function.
	rps := make(map[string]float32)
	for funcName := range funcs {
		rps[funcName] = iterationRate
	}

	if len(rps) == 0 {
		logger.Warn("No deployed functions returned, skipping message publishing")
		return nil
	}

	logger.Debugw("Retrieved k6 input rate",
		"rate", iterationRate,
		"functions_count", len(rps),
	)

	// There may be cases where the function returned by the deployed functions
	// differs from the functions available in the proxy. In these cases, we
	// simply skip the function. It will become available at some point.
	overloaded := make(map[string]bool)
	for funcName, funcRPS := range rps {
		limit, present := funcs[funcName]
		if !present {
			continue
		}
		overloaded[funcName] = funcRPS >= float32(limit)
	}

	// This is what will be shared with all other neighbors.
	limits := make(map[string]map[string]float32)
	for _, neighborID := range neighborsID {
		limits[neighborID] = make(map[string]float32)
	}

	// Calculate limits to share.
	for funcName, funcOverloaded := range overloaded {
		if funcOverloaded {
			// Overloaded: none of the neighbors should send anything to me.
			for _, neighborID := range neighborsID {
				limits[neighborID][funcName] = 0
			}

			logger.Debugw("Function overloaded: disabling incoming neighbor traffic",
				"function", funcName,
				"current_rps", rps[funcName],
				"max_rps", funcs[funcName],
			)
			continue
		}

		// Not overloaded means we have enough margin to allow other neighbors
		// to send requests to this node.
		limit := funcs[funcName]
		funcRPS := rps[funcName]
		margin := float32(limit) - funcRPS
		if margin < 0 {
			logger.Warnf("Unexpected negative margin, will be set to 0: %f", margin)
			margin = 0
		}

		// The available margin will be evenly distributed among the neighbors.
		share := margin / float32(len(neighborsID))
		for _, neighborID := range neighborsID {
			limits[neighborID][funcName] = share
		}

		logger.Debugw("Function not overloaded: distributed available capacity among neighbors",
			"function", funcName,
			"current_rps", funcRPS,
			"max_rps", limit,
			"available_margin", margin,
			"share_per_neighbor", share,
			"neighbors_count", len(neighborsID),
		)
	}

	// Finally we can send the message.
	msg := MsgRecalc{MsgType: MsgTypeRecalc, FuncLimits: limits}
	if err := communication.MarshAndPublish(msg); err != nil {
		return fmt.Errorf("failed to publish node info to neighbors: %w", err)
	}
	return nil
}

func (strategy *RecalcStrategy) updateLocalWeights() error {
	logger := logging.Logger().With("strategy_component", "updateLocalWeights")

	// We get the incoming request rate directly from the proxy via the Runtime
	// API, rather than through Prometheus, because this method is expected to
	// run within a few seconds (<5) of the start of a k6 iteration. Prometheus
	// scraping is not frequent enough, so we may miss some data.
	proxyReqRate, err := strategy.runtimeapi.ReqRate()
	if err != nil {
		return fmt.Errorf("failed to get proxy stats: %w", err)
	}
	logger.Debugw("Proxy req rate stats", "data", proxyReqRate)

	funcs, err := strategy.funcsClient.WithMaxRates()
	if err != nil {
		return fmt.Errorf("failed to get deployed functions: %w", err)
	}

	neighborsID := []string{} // Node's ID (node_XXX).
	for _, peer := range _p2pHost.Network().Peers() {
		// "node_ID" is required format.
		peerID := fmt.Sprintf("node_%s", peer)
		neighborsID = append(neighborsID, peerID)
	}

	backends, err := strategy.runtimeapi.Backends()
	if err != nil {
		return fmt.Errorf("failed to get proxy current backends: %w", err)
	}

	logger.Debugw("Retrieved system and proxy state",
		"functions_count", len(funcs),
		"neighbors_count", len(neighborsID),
		"proxy_backends_count", len(backends),
	)

	// Before setting the weights, we need to know the current incoming RPS from
	// clients and other neighbors. We retrieve it directly from HAProxy instead
	// of querying Prometheus.
	rps := make(map[string]float32)
	neighborsRPS := make(map[string]map[string]float32)

	for funcName := range funcs {
		backend := "function_" + funcName
		if servers, present := proxyReqRate[backend]; present {
			rps[funcName] = servers["BACKEND"]
		} else {
			rps[funcName] = 0
		}
	}

	for _, neighborID := range neighborsID {
		neighborsRPS[neighborID] = make(map[string]float32)

		for funcName := range funcs {
			backend := fmt.Sprintf("function_%s_forwarded_%s", funcName, neighborID)
			if servers, present := proxyReqRate[backend]; present {
				neighborsRPS[neighborID][funcName] = servers["BACKEND"]
			} else {
				neighborsRPS[neighborID][funcName] = 0
			}
		}
	}

	// The weights map contains the raw weights to apply to each server for each
	// backend in the proxy. We first initialize the map and its submaps; they
	// will be populated by the code that follows.
	weights := make(map[string]map[string]uint)
	for funcName := range funcs {
		weights["function_"+funcName] = make(map[string]uint)
		for _, neighborID := range neighborsID {
			backend := fmt.Sprintf("function_%s_forwarded_%s", funcName, neighborID)
			weights[backend] = make(map[string]uint)
		}
	}

	for funcName, funcRPS := range rps {
		limit := funcs[funcName]
		overloaded := funcRPS >= float32(limit)

		logger.Debugw("Evaluating function load",
			"function", funcName,
			"current_rps", funcRPS,
			"max_rps", limit,
			"overloaded?", overloaded,
		)

		if overloaded {
			// We first disable incoming requests from neighbors because we are
			// overloaded.
			for _, neighborID := range neighborsID {
				backend := fmt.Sprintf("function_%s_forwarded_%s", funcName, neighborID)
				weights[backend]["faas-local"] = 0
				weights[backend]["rejector"] = 100
			}

			// We need to prepare the limits map to pass to the calculateWeights
			// method. This map defines the RPS limits that the local node and
			// its neighbors can handle.
			limits := make(map[string]float32)
			limits["faas-local"] = float32(limit)
			strategy.limitsTableLock.RLock()
			for neighborID := range strategy.limitsTable {
				// In the limitsTable neighbors ID are stored as plain ID, but in
				// the proxy we need to provide the "node_ID" form.
				neighborNodeID := fmt.Sprintf("node_%s", neighborID)

				neighborLimit, present := strategy.limitsTable[neighborID][funcName]
				if !present || math.IsNaN(float64(neighborLimit)) || math.IsInf(float64(neighborLimit), 0) {
					logger.Warnf("Neighbor %q for function %q has a limit of 0, NaN or Inf: %f", neighborID, funcName, neighborLimit)
					limits[neighborNodeID] = 0
				} else {
					logger.Debugw("Using neighbor capacity",
						"function", funcName,
						"neighbor_id", neighborNodeID,
						"neighbor_limit_rps", neighborLimit,
					)
					limits[neighborNodeID] = neighborLimit
				}
			}
			strategy.limitsTableLock.RUnlock()

			funcWeights := strategy.calculateWeights(funcRPS, limits)

			logger.Debugw("Calculated proxy weights for overloaded function",
				"function", funcName,
				"current_rps", funcRPS,
				"limits_rps", limits,
				"calculated_proxy_weights", funcWeights,
			)

			// We now have the weights, but we need to store them in the global
			// weight table.
			backend := fmt.Sprintf("function_%s", funcName)
			for server, weight := range funcWeights {
				weights[backend][server] = weight
			}
			continue
		}

		// Since the node is not overloaded for this function, we can process
		// all requests locally.
		backend := fmt.Sprintf("function_%s", funcName)
		weights[backend]["faas-local"] = 100
		weights[backend]["rejector"] = 0
		for _, neighborID := range neighborsID {
			weights[backend][neighborID] = 0
		}

		// No neighbors, nothing to do.
		if len(neighborsID) == 0 {
			continue
		}

		// We have an RPS margin that we can accept from neighbors.
		margin := float32(limit) - funcRPS
		if margin < 0 {
			logger.Warnf("Unexpected negative margin, will be set to 0: %f", margin)
			margin = 0
		}

		share := margin / float32(len(neighborsID))
		for _, neighborID := range neighborsID {
			backend := fmt.Sprintf("function_%s_forwarded_%s", funcName, neighborID)
			neighborRPS := neighborsRPS[neighborID][funcName]

			var allowed uint
			var decision string // Only for debug.

			// When calculating the weights, we also need to check the current
			// incoming RPS, as it may exceed the margin assigned to that
			// neighbor. In that case, some requests must be rejected.
			switch {
			case neighborRPS <= 0:
				allowed = 100
				decision = "no_incoming_traffic"

			case share <= 0:
				allowed = 0
				decision = "no_available_capacity"

			case neighborRPS <= share:
				allowed = 100
				decision = "incoming_traffic_within_capacity"

			default:
				allowed = uint(math.Round(float64(share) / float64(neighborRPS) * 100))
				decision = "incoming_traffic_over_capacity"
			}

			rejected := 100 - allowed

			weights[backend]["faas-local"] = allowed
			weights[backend]["rejector"] = rejected

			logger.Debugw("Calculated forwarded traffic weights",
				"function", funcName,
				"decision", decision,
				"neighbor_id", neighborID,
				"neighbor_rps", neighborRPS,
				"available_share_rps", share,
				"allowed_proxy_weight", allowed,
				"rejected_proxy_weight", rejected,
			)
		}
	}

	// Apply all weights.
	for backend := range weights {
		for server, weight := range weights[backend] {
			if err := strategy.runtimeapi.SetWeight(backend, server, weight); err != nil {
				return fmt.Errorf("failed to set weight %d for server %q on backend %q: %w", weight, server, backend, err)
			}
		}
	}
	logger.Debugw("Proxy weights updated", "weights", weights)

	return nil
}

// processMsgRecalc processes an incoming message received from another node.
func (strategy *RecalcStrategy) processMsgRecalc(senderID string, msg *MsgRecalc) error {
	logger := logging.Logger().With("strategy_component", "processMsgRecalc")
	myselfID := _p2pHost.ID().String()

	if senderID == myselfID {
		return nil // Ignore ourselves
	}

	// All nodes inside the message are referenced as "node_X", where X is the
	// full node ID.
	myselfID = fmt.Sprintf("node_%s", myselfID)

	// If the sender node does not "know" us (we aren't in his FuncLimits) we just
	// ignore the message.
	funcLimits, present := msg.FuncLimits[myselfID]
	if !present {
		logger.Debugf("Received message from %s, but local node is not present, skipping message", senderID)
		return nil
	}

	strategy.limitsTableLock.Lock()
	defer strategy.limitsTableLock.Unlock()

	// If the neighbor is new, maybe not present in the local table, in that
	// case we need to initialize the entry.
	//
	// FIXME: In this way a removed neighbor will be kept in the table!
	if _, present := strategy.limitsTable[senderID]; !present {
		strategy.limitsTable[senderID] = make(map[string]float32)
	}

	// Now we can overwrite (or set initially) the limits.
	//
	// FIXME: In this way a removed function will be kept in the table!
	for funcName, limits := range funcLimits {
		logger.Debugf("Received message from %s, for function %q local node can send max. %.2f req/s", senderID, funcName, limits)
		strategy.limitsTable[senderID][funcName] = limits
	}

	return nil
}

// calculateWeights calculates integer proxy weights for a function.
//
// limits contains the RPS limits for "faas-local" and zero or more
// neighbors.
//
// If currentRPS is below the local limit, all traffic is assigned locally.
// Otherwise, local capacity is saturated first, then remaining traffic is
// distributed proportionally to the neighbors' remaining capacities without
// exceeding their limits. Unhandled traffic and rounding remainders are
// assigned to "rejector".
//
// The returned map includes all input keys plus "rejector". Weights always sum
// to 100.
func (strategy *RecalcStrategy) calculateWeights(currentRPS float32, limits map[string]float32) map[string]uint {
	const (
		local    = "faas-local"
		rejector = "rejector"
	)

	// Initialize output weights.
	result := make(map[string]uint, len(limits)+1)
	for server := range limits {
		result[server] = 0
	}
	result[local] = 100 // By default process all locally.
	result[rejector] = 0

	localLimit := max(0, limits[local])
	if currentRPS == 0 || currentRPS <= localLimit {
		// Below the local limit, handle all traffic locally.
		return result
	}

	// We are now in the overloaded case. We first saturate the local node and
	// calculate the remaining traffic.
	localRPS := min(localLimit, currentRPS)
	remaining := currentRPS - localRPS

	assignedRPS := make(map[string]float32, len(limits))
	assignedRPS[local] = localRPS

	// Include neighbors with a positive capacity, we know they're not
	// overloaded.
	neighbors := make([]string, 0, len(limits))
	for neighborID, limit := range limits {
		if neighborID != local && limit > 0 {
			neighbors = append(neighbors, neighborID)
		}
	}

	// We distribute the remaining traffic proportionally among the available
	// neighbors. The complexity comes from rounding when converting from float
	// to uint.
	for remaining > 0 && len(neighbors) > 0 {
		var totalCapacity float32
		for _, neighborID := range neighbors {
			totalCapacity += limits[neighborID] - assignedRPS[neighborID]
		}

		if totalCapacity <= 0 {
			break
		}

		distributed := float32(0)
		availableNeighbors := make([]string, 0, len(neighbors))

		for _, neighborID := range neighbors {
			// Calculate a proportional share, capped by the neighbor's
			// remaining capacity.
			capacity := limits[neighborID] - assignedRPS[neighborID]
			share := min(remaining*capacity/totalCapacity, capacity)

			assignedRPS[neighborID] += share
			distributed += share

			// Keep neighbors that still have available capacity.
			if limits[neighborID]-assignedRPS[neighborID] > 0 {
				availableNeighbors = append(availableNeighbors, neighborID)
			}
		}

		// Avoid an infinite loop if no further traffic can be assigned.
		if distributed <= 0 {
			break
		}

		remaining -= distributed
		neighbors = availableNeighbors
	}

	// Convert assigned RPS to integer percentages.
	var totalWeight uint
	for server, rate := range assignedRPS {
		weight := uint(math.Round(float64(rate) / float64(currentRPS) * 100))
		result[server] = weight
		totalWeight += weight
	}

	// Reject unhandled. Note that the totalWeight could be greater than 100 due
	// to rounding differences, in that case do not reject anything.
	if totalWeight < 100 {
		result[rejector] = 100 - totalWeight
	}

	return result
}

// setAllTrafficLocal resets the proxy weights so that client traffic is handled
// locally and forwarded traffic is rejected.
func (strategy *RecalcStrategy) setAllTrafficLocal() error {
	logger := logging.Logger().With("strategy_component", "setAllTrafficLocal")

	// Get deployed functions.
	funcs, err := strategy.funcsClient.WithMaxRates()
	if err != nil {
		return fmt.Errorf("failed to get deployed functions: %w", err)
	}

	// Get the current proxy backends.
	backends, err := strategy.runtimeapi.Backends()
	if err != nil {
		return fmt.Errorf("failed to get proxy current backends: %w", err)
	}

	// Collect neighbors.
	neighborsID := []string{}
	for _, peer := range _p2pHost.Network().Peers() {
		// "node_ID" is required format.
		peerID := fmt.Sprintf("node_%s", peer)
		neighborsID = append(neighborsID, peerID)
	}

	for funcName := range funcs {
		// All client requests are handled locally with no rejections and no
		// forwarding.
		localBackend := fmt.Sprintf("function_%s", funcName)
		if slices.Contains(backends, localBackend) {
			if err := strategy.runtimeapi.SetWeight(localBackend, "faas-local", 100); err != nil {
				return fmt.Errorf("failed to set local weight for %q: %w", localBackend, err)
			}

			if err := strategy.runtimeapi.SetWeight(localBackend, "rejector", 0); err != nil {
				return fmt.Errorf("failed to reset rejector weight for %q: %w", localBackend, err)
			}

			for _, neighborID := range neighborsID {
				if err := strategy.runtimeapi.SetWeight(localBackend, neighborID, 0); err != nil {
					return fmt.Errorf("failed to disable neighbor %q on %q: %w", neighborID, localBackend, err)
				}
			}
		}

		// Forwarded traffic is rejected until the next iteration is configured.
		for _, neighborID := range neighborsID {
			forwardedBackend := fmt.Sprintf("function_%s_forwarded_%s", funcName, neighborID)

			if !slices.Contains(backends, forwardedBackend) {
				continue
			}

			if err := strategy.runtimeapi.SetWeight(forwardedBackend, "faas-local", 0); err != nil {
				return fmt.Errorf("failed to disable forwarded backend %q: %w", forwardedBackend, err)
			}

			if err := strategy.runtimeapi.SetWeight(forwardedBackend, "rejector", 100); err != nil {
				return fmt.Errorf("failed to reject forwarded traffic on %q: %w", forwardedBackend, err)
			}
		}
	}

	logger.Info("Proxy weights reset: all client traffic is local")
	return nil
}

func (strategy *RecalcStrategy) reloadProxy() error {
	neighborsID := []string{}                // Node's ID (node_XXX).
	neighborsHost := make(map[string]string) // Node's ID -> Node's host addr.
	for _, peer := range _p2pHost.Network().Peers() {
		host, err := extractSingleIPv4(_p2pHost, peer)
		if err != nil {
			return fmt.Errorf("failed to build neighbors information: %w", err)
		}

		// "node_ID" is required format.
		peerID := fmt.Sprintf("node_%s", peer)

		neighborsID = append(neighborsID, peerID)
		neighborsHost[peerID] = host
	}

	funcsTimeout, err := strategy.funcsClient.WithTimeout()
	if err != nil {
		return fmt.Errorf("get function metadata: %w", err)
	}

	// Add 1 seconds to base timeout (if given) to all functions.
	for _, timeout := range funcsTimeout {
		if timeout != nil {
			*timeout += 1000
		}
	}

	// Define and populate this anonymous struct to share data to the Go
	// template that will build the proxy configuration. See
	// haproxycfgrecalc.tmpl file.
	data := struct {
		Now           string
		DFaaSNodeID   string
		Functions     map[string]*uint
		Neighbors     []string
		NeighborsHost map[string]string
		HAProxyPort   uint
		FaaSHost      string
		FaaSPort      uint
		RejectorHost  string
		RejectorPort  uint
	}{
		Now:           time.Now().UTC().Format("2006-01-02 15:04:05 MST"),
		DFaaSNodeID:   _p2pHost.ID().String(),
		Functions:     funcsTimeout,
		Neighbors:     neighborsID,
		NeighborsHost: neighborsHost,
		HAProxyPort:   _config.HAProxyPort,
		FaaSHost:      _config.FaaSHost,
		FaaSPort:      _config.FaaSPort,
		RejectorHost:  _config.RejectorHost,
		RejectorPort:  _config.RejectorPort,
	}

	// The data structure will be passed to the hacfgupdater module and a
	// new HAProxy config. will be generated and passed to the proxy. The
	// proxy will restart.
	return strategy.hacfgupdater.UpdateHAConfig(data)
}

// sameKeys returns true if the given dicts have the same keys, otherwise
// returns false.
func sameKeys[K comparable, V1, V2 any](a map[K]V1, b map[K]V2) bool {
	if len(a) != len(b) {
		return false
	}

	for k := range a {
		if _, present := b[k]; !present {
			return false
		}
	}

	return true
}

func (strategy *RecalcStrategy) waitForProxyVariable(variable string) (string, error) {
	for {
		value, found, err := strategy.runtimeapi.Variable(variable)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", variable, err)
		}

		if found {
			return value, nil
		}

		time.Sleep(time.Second)
	}
}
