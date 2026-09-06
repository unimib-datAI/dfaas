// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2pping "github.com/libp2p/go-libp2p/p2p/protocol/ping"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/httpserver"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
)

// LatencyThresholdStrategy implements a latency-aware routing load balancing
// strategy. It measures the latency of currently connected libp2p neighbours,
// filters them using the configured latency threshold, and distributes traffic
// equally between the local node and all eligible neighbours.
type LatencyThresholdStrategy struct {
	hacfgupdater  *hacfgupd.Updater
	offuncsClient *offuncs.Client

	thresholdMs float64
}

// RunStrategy handles the periodic execution of the recalculation function.
// It should run in a goroutine.
func (strategy *LatencyThresholdStrategy) RunStrategy() error {
	logger := logging.Logger()
	logger.Infof("Staring LatencyThreshold strategy with threshold of %.2fms", strategy.thresholdMs)

	ticker := time.NewTicker(_config.RecalcPeriod)
	defer ticker.Stop()

	for range ticker.C {
		start := time.Now().UTC()

		weights, err := strategy.calculateWeights()
		if err != nil {
			return fmt.Errorf("calculating new weights: %w", err)
		}

		if err = strategy.setProxyWeights(weights); err != nil {
			return fmt.Errorf("setting new weights: %w", err)
		}

		duration := time.Since(start)
		httpserver.StrategyIterationDuration.Set(duration.Seconds())
		httpserver.StrategySuccessIterations.Inc()

		logger.Infof("Iteration completed. Duration: %s", duration.String())
	}

	return nil
}

// OnReceived is executed every time a message from a peer is received.
func (strategy *LatencyThresholdStrategy) OnReceived(msg *pubsub.Message) error {
	return nil
}

// In LatencyThreshold strategy, requests are distributed equally between the
// local node and all currently connected libp2p neighbours that have at least
// one function in common and satisfy the latency threshold. If there are no
// usable neighbours, all requests are processed locally.
func (strategy *LatencyThresholdStrategy) calculateWeights() (map[string]map[string]uint, error) {
	logger := logging.Logger()

	myNodeID := _p2pHost.ID().String()

	// weights maps each function name to its load-balancing weights for each
	// neighbor, including the node itself. Each weight represents the
	// percentage of requests to forward to that node.
	weights := make(map[string]map[string]uint)

	// Iterate and filter over peers that are currently connected to this libp2p
	// host.
	var usableNeighbours []string
	for _, p := range _p2pHost.Network().Peers() {
		neighID := p.String()

		// Never consider ourselves as a neighbour.
		if p == _p2pHost.ID() {
			continue
		}

		latencyMs, err := getLatencyMs(neighID)
		if err != nil {
			logger.Warnf("Excluding node %s because libp2p ping failed: %v", neighID, err)
			continue
		}
		logger.Infof("Measured latency for node %s: %.2fms", neighID, latencyMs)

		if latencyMs > strategy.thresholdMs {
			logger.Infof("Node %s excluded since latency is over threshold %fms", neighID, strategy.thresholdMs)
			continue
		}

		usableNeighbours = append(usableNeighbours, neighID)
	}

	// Get the list of deployed OpenFaaS functions.
	funcs, err := strategy.offuncsClient.GetFuncsNames()
	if err != nil {
		return nil, fmt.Errorf("failed to check deployed functions: %w", err)
	}
	if len(funcs) == 0 {
		logger.Error("Strategy requires at least one deployed function, found none")
		return nil, fmt.Errorf("at least one function required, found %d", len(funcs))
	}

	for _, funcName := range funcs {
		weights[funcName] = make(map[string]uint)

		const totalWeight uint = 100

		if len(usableNeighbours) == 0 {
			weights[funcName][myNodeID] = 100
			logger.Infof("For function %s no usable neighbours, routing 100%% local", funcName)
			continue
		}

		totalNodes := uint(len(usableNeighbours) + 1)
		baseWeight := totalWeight / totalNodes
		remainder := totalWeight % totalNodes

		// Local node gets one extra point if there is a remainder.
		weights[funcName][myNodeID] = baseWeight
		if remainder > 0 {
			weights[funcName][myNodeID]++
		}

		for i, neighbour := range usableNeighbours {
			weight := baseWeight

			// Give the remaining weight points to neighbours after
			// the local node has received its extra point.
			if uint(i) < remainder-1 {
				weight++
			}

			weights[funcName][neighbour] = weight
		}
	}

	debugWeightsNMS(weights)

	return weights, nil
}

// Update HAProxy configuration with new weights.
func (strategy *LatencyThresholdStrategy) setProxyWeights(weights map[string]map[string]uint) error {
	// Get the deployed functions and their timeout configuration.
	funcs, err := strategy.offuncsClient.GetFuncsWithTimeout()
	if err != nil {
		return fmt.Errorf("get function metadata: %w", err)
	}

	// Add 1 second to the configured timeout of all functions.
	for _, timeout := range funcs {
		if timeout != nil {
			*timeout += 1000
		}
	}

	// Get neighbours information with IP and port connection.
	var neighbors []string

	neighborsPort := make(map[string]string)
	neighborsHost := make(map[string]string)

	for _, p := range _p2pHost.Network().Peers() {
		host, err := extractSingleIPv4(_p2pHost, p)
		if err != nil {
			return fmt.Errorf("failed to build neighbors information: %w", err)
		}

		peerID := p.String()

		neighbors = append(neighbors, peerID)
		neighborsHost[peerID] = host
		// FIXME: The remote proxy port may be different from local proxy port.
		neighborsPort[peerID] = strconv.FormatUint(uint64(_config.HAProxyPort), 10)
	}

	// We need to pass a lot of information because Go templates have a limited
	// set of functions for extracting and manipulating data.
	data := struct {
		Now           string
		DFaaSNodeID   string
		Functions     map[string]*uint
		Neighbors     []string
		NeighborsPort map[string]string
		NeighborsHost map[string]string
		Weights       map[string]map[string]uint
		OpenFaaSHost  string
		OpenFaaSPort  uint
	}{
		Now:           time.Now().UTC().Format("2006-01-02 15:04:05 MST"),
		DFaaSNodeID:   _p2pHost.ID().String(),
		Functions:     funcs,
		Neighbors:     neighbors,
		NeighborsHost: neighborsHost,
		NeighborsPort: neighborsPort,
		Weights:       weights,
		OpenFaaSHost:  _config.OpenFaaSHost,
		OpenFaaSPort:  _config.OpenFaaSPort,
	}

	return strategy.hacfgupdater.UpdateHAConfig(data)
}

// getLatencyMs takes a peer ID string and returns the peer's average round-trip
// latency in milliseconds. It uses three pings to get a more stable result.
//
// TODO: Run the ping in a dedicated goroutine so a slow node does not block the
// strategy cycle. A 500 ms ping currently wastes 500 ms of the cycle.
func getLatencyMs(nodeID string) (float64, error) {
	p, err := peer.Decode(nodeID)
	if err != nil {
		return 0, fmt.Errorf("invalid peer id %s: %w", nodeID, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := libp2pping.Ping(ctx, _p2pHost, p)

	var totalMs float64
	var count int

	for count < 3 {
		select {
		case res, ok := <-ch:
			if !ok {
				if count == 0 {
					return 0, fmt.Errorf("libp2p ping channel closed before receiving results")
				}
				return totalMs / float64(count), nil
			}

			if res.Error != nil {
				return 0, fmt.Errorf("libp2p ping error for peer %s: %w", nodeID, res.Error)
			}

			totalMs += float64(res.RTT) / float64(time.Millisecond)
			count++

		case <-ctx.Done():
			if count == 0 {
				return 0, fmt.Errorf("libp2p ping timeout for peer %s", nodeID)
			}
			return totalMs / float64(count), nil
		}
	}

	return totalMs / float64(count), nil
}
