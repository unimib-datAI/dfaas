// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/promq"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/proxy"
)

// This file contains the implementation of the Recalc strategy.

// RecalcStrategy is the structure that implements the Strategy interface.
//
// Internally it works with 3 indipendents running components:
//
// 1. A component that receives and process messages coming from other nodes,
// updating the table of Limits. That the "OnReceived" method.
//
// 2. A component that, periodically, calculate how much can receive from
// neighbors and publish the information as a message.
//
// 3. A component that, periodically, update the HAProxy weights based on the
// current status (Limits, nr. of requests per second...).
//
// The 2nd and 3rd components are launched in a dedicated goroutines by the
// "RunStrategy" method.
//
// See also the corresponding createStrategy method to understand how the struct
// fields are initialized.
type RecalcStrategy struct {
	hacfgupdater  *hacfgupd.Updater
	offuncsClient *offuncs.Client
	runtimeapi    *proxy.RuntimeAPI
	promq         *promq.Client

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

// RunStrategy handles the periodic execution of the recalculation function. It
// should run in a goroutine.
func (strategy *RecalcStrategy) RunStrategy() error {
	logger := logging.Logger()

	// We first reload the proxy to apply the custom HAProxy default config.
	if err := strategy.reloadProxy(); err != nil {
		return err
	}

	// We run two goroutines, each with its own responsibility and timing. This
	// function prepares the tickers for both goroutines, starts them, and then
	// waits for the termination signal.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	limitsPeriod := _config.RecalcPeriod

	// We use the Prometheus step as the period for the weight update mechanism
	// because it runs a Prometheus query, which would return useless data if
	// executed at a finer granularity than the Prometheus step, since no new
	// data would be available.
	weightsPeriod := _config.PrometheusStep

	var wg sync.WaitGroup

	wg.Go(func() {
		ticker := time.NewTicker(limitsPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := strategy.updateLimitsForNeighbors(); err != nil {
					logger.Errorf(
						"Failed to prepare and publish the message to neighbors: %v",
						err,
					)
				}
			}
		}
	})

	wg.Go(func() {
		ticker := time.NewTicker(weightsPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := strategy.updateLocalWeights(); err != nil {
					logger.Errorf(
						"Failed to update the local weights: %v",
						err,
					)
				}
			}
		}
	})

	// Wait for interruption...
	<-ctx.Done()
	logger.Info("Stopping recalculation strategy...")
	stop()
	wg.Wait()
	logger.Info("Recalculation strategy stopped")

	return nil
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
		logging.Logger().Warnf("Skipping message, failed to parse as MsgNodeInfoRecalc type: %w", err)
		return fmt.Errorf("error while deserializing a message: %w", err)
	}

	strategy.processMsgRecalc(msg.GetFrom().String(), &parsedMsg)

	return nil
}

func (strategy *RecalcStrategy) updateLimitsForNeighbors() error {
	var err error
	logger := logging.Logger()

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

	// Get deployed functions with associated limits.
	funcs, err := strategy.offuncsClient.GetFuncsWithMaxRates()
	if err != nil {
		return fmt.Errorf("failed to get deployed functions: %w", err)
	}

	// Get the input data only from a Prometheus query.
	end := time.Now().UTC()
	start := end.Add(-_config.RecalcPeriod)
	rps, err := strategy.promq.InputRPS(start, end)
	if err != nil {
		return fmt.Errorf("failed to run InputRPS query: %w", err)
	}
	if len(rps) == 0 {
		// This can happen when the system has just started up.
		logger.Warn("No functions returned from InputRPS query, skipping message publishing")
		return nil
	}

	// There may be cases where the function returned by the Prometheus query
	// differs from the deployed functions because a function has not yet been
	// added to the proxy layer. In these cases, we simply skip the function. It
	// will become available at some point.
	overloaded := make(map[string]bool)
	for funcName, funcRPS := range rps {
		overloaded[funcName] = false

		limit := funcs[funcName]
		overloaded[funcName] = funcRPS >= float32(limit)
	}

	// This is what will be shared with all other neighbors.
	limits := make(map[string]map[string]float32)
	for _, neighborID := range neighborsID {
		limits[neighborID] = make(map[string]float32)
	}

	// Calculate limits to share
	for funcName, funcOverloaded := range overloaded {
		if funcOverloaded {
			// Overloaded: none of the neighbors should send anything to me.
			for _, neighborID := range neighborsID {
				limits[neighborID][funcName] = 0
			}
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
	}

	// Finally we can send the message.
	msg := MsgRecalc{MsgType: MsgTypeRecalc, FuncLimits: limits}
	if err := communication.MarshAndPublish(msg); err != nil {
		return fmt.Errorf("failed to publish node info to neighbors: %w", err)
	}
	return nil
}

func (strategy *RecalcStrategy) updateLocalWeights() error {
	logger := logging.Logger()

	// Before updating the proxy weights, the proxy must be reloaded whenever a
	// function or a neighbor is added or removed. To check whether a reload is
	// needed, we must compare the detected neighbors and functions with the
	// proxy backends.

	funcs, err := strategy.offuncsClient.GetFuncsWithMaxRates()
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

	reload := false
loop:
	for funcName := range funcs {
		if !slices.Contains(backends, "function_"+funcName) {
			// Missing backend for handling this function from clients.
			reload = true
			break
		}
		for _, neighborID := range neighborsID {
			bkd := fmt.Sprintf("function_%s_forwarded_%s", funcName, neighborID)
			if !slices.Contains(backends, bkd) {
				// Missing backend for handling these functions from other
				// neighbors.
				reload = true
				break loop
			}
		}
	}

	// BUG: This code assumes that proxy reloads happen very infrequently. When
	// the proxy is reloaded, the weights are reset to their default values and
	// then updated. However, in the meantime, requests may arrive while the
	// proxy is still using the reset weights.
	if reload {
		// BUG: After the reload, we cannot update the weights immediately
		// because we need to wait for the reload to complete. Since we cannot
		// determine this for now, we simply skip this iteration.
		return strategy.reloadProxy()
	}

	// Before setting the weights, we need to know the current incoming RPS from
	// clients and other neighbors.

	end := time.Now().UTC()
	// We double the interval to make sure we capture at least 2 data points;
	// otherwise, the query can return empty.
	start := end.Add(-(_config.PrometheusStep * 2))
	rps, err := strategy.promq.InputRPS(start, end)
	if err != nil {
		return fmt.Errorf("failed to run InputRPS query: %w", err)
	}
	if len(rps) == 0 {
		// This can happen when the system has just started up.
		logger.Warn("No functions returned from InputRPS query, skipping updating proxy weights.")
		return nil
	}

	neighborsRPS, err := strategy.promq.ForwardedRPS(start, end)
	if err != nil {
		return fmt.Errorf("failed to run ForwardedRPS query: %w", err)
	}

	// The weights map contains the raw weights to apply to each server for each
	// backend in the proxy. We first initialize the map and its submaps; they
	// will be populated by the code that follows.
	weights := make(map[string]map[string]uint)
	for funcName := range rps {
		weights["function_"+funcName] = make(map[string]uint)
		for _, neighborID := range neighborsID {
			backend := fmt.Sprintf("function_%s_forwarded_%s", funcName, neighborID)
			weights[backend] = make(map[string]uint)
		}
	}

	for funcName, funcRPS := range rps {
		limit := funcs[funcName]
		if funcRPS >= float32(limit) {
			// We first disable incoming requests from neighbors because we are
			// overloaded.
			for _, neighborID := range neighborsID {
				backend := fmt.Sprintf("function_%s_forwarded_%s", funcName, neighborID)
				weights[backend]["openfaas-local"] = 0
				weights[backend]["rejector"] = 100
			}

			// We need to prepare the limits map to pass to the calculateWeights
			// method. This map defines the RPS limits that the local node and
			// its neighbors can handle.
			limits := make(map[string]float32)
			limits["openfaas-local"] = float32(limit)
			strategy.limitsTableLock.RLock()
			for neighborID := range strategy.limitsTable {
				// In the limitsTable neighbors ID are store as plain ID, but in
				// the proxy we need to provide the "node_ID" form.
				neighborNodeID := fmt.Sprintf("node_%s", neighborID)

				neighborLimit, present := strategy.limitsTable[neighborID][funcName]
				if !present || math.IsNaN(float64(neighborLimit)) || math.IsInf(float64(neighborLimit), 0) {
					logger.Warnf("Neighbor %q for function %q has a limit of 0, NaN or Inf: %f", neighborID, funcName, neighborLimit)
					limits[neighborNodeID] = 0
				} else {
					limits[neighborNodeID] = neighborLimit
				}
			}
			strategy.limitsTableLock.RUnlock()

			funcWeights := strategy.calculateWeights(funcRPS, limits)

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
		weights[backend]["openfaas-local"] = 100
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

		// The available margin will be evenly distributed among the neighbors.
		share := margin / float32(len(neighborsID))
		for _, neighborID := range neighborsID {
			backend := fmt.Sprintf("function_%s_forwarded_%s", funcName, neighborID)

			// When calculating the weights, we also need to check the current
			// incoming RPS, as it may exceed the margin assigned to that
			// neighbor. In that case, some requests must be rejected.
			neighborRPS := neighborsRPS[neighborID][funcName]
			if neighborRPS <= 0 {
				weights[backend]["openfaas-local"] = 100
				weights[backend]["rejector"] = 0
				continue
			}

			if share <= 0 {
				weights[backend]["openfaas-local"] = 0
				weights[backend]["rejector"] = 100
				continue
			}

			if neighborRPS <= share {
				weights[backend]["openfaas-local"] = 100
				weights[backend]["rejector"] = 0
				continue
			}

			allowed := uint(share / neighborRPS * 100)
			weights[backend]["openfaas-local"] = allowed
			weights[backend]["rejector"] = 100 - allowed
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
	logger.Debugf("HAProxy updated with the following weights: %v", weights)

	return nil
}

// processMsgRecalc processes an incoming message received from another node.
func (strategy *RecalcStrategy) processMsgRecalc(senderID string, msg *MsgRecalc) error {
	myselfID := _p2pHost.ID().String()

	if senderID == myselfID {
		return nil // Ignore ourselves
	}

	// All nodes inside the message are refereced as "node_X", where X is the
	// full node ID.
	myselfID = fmt.Sprintf("node_%s", myselfID)

	// If the sender node do not "know" us (we aren't in his FuncLimits) we just
	// ignore the message.
	funcLimits, present := msg.FuncLimits[myselfID]
	if !present {
		return nil
	}

	strategy.limitsTableLock.Lock()
	defer strategy.limitsTableLock.Unlock()

	// If the neighbor is new, may be not present in the local table, in that
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
		strategy.limitsTable[senderID][funcName] = limits
	}

	return nil
}

// calculateWeights calculates integer proxy weights for a function.
//
// limits contains the RPS limits for "openfaas-local" and zero or more
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
		local    = "openfaas-local"
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

	// Convert assigned RPS to integer percentages, rounding down.
	var totalWeight uint
	for server, rate := range assignedRPS {
		weight := uint(rate / currentRPS * 100)
		result[server] = weight
		totalWeight += weight
	}

	// Reject unhandled traffic and absorb percentage rounding differences.
	if totalWeight < 100 {
		result[rejector] = 100 - totalWeight
	}

	return result
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

	funcsTimeout, err := strategy.offuncsClient.GetFuncsWithTimeout()
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
		OpenFaaSHost  string
		OpenFaaSPort  uint
		RejectorHost  string
		RejectorPort  uint
	}{
		Now:           time.Now().UTC().Format("2006-01-02 15:04:05 MST"),
		DFaaSNodeID:   _p2pHost.ID().String(),
		Functions:     funcsTimeout,
		Neighbors:     neighborsID,
		NeighborsHost: neighborsHost,
		HAProxyPort:   _config.HAProxyPort,
		OpenFaaSHost:  _config.OpenFaaSHost,
		OpenFaaSPort:  _config.OpenFaaSPort,
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
		if _, ok := b[k]; !ok {
			return false
		}
	}

	return true
}
