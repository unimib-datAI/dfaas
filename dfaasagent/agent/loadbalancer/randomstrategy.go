// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/httpserver"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/proxy"
)

type RandomStrategy struct {
	// HAProxy Data Plane API client used to generate and update the HAProxy
	// configuration.
	hacfgupdater *hacfgupd.Updater

	// HAProxy Runtime API client used to update runtime configuration without
	// reloading HAProxy.
	runtimeapi *proxy.RuntimeAPI

	// OpenFaaS Gateway client used to get the list of deployed functions.
	offuncsClient *offuncs.Client

	// Starting seed for the pseudo-random number generator. If set to -1, a
	// random seed is used.
	randomSeed int64

	// If true, the strategy can randomly reject requests. If false, requests
	// are only processed locally or forwarded to neighbors.
	randomReject bool

	// Pseudo-random number generator used to generate the request distribution.
	random *rand.Rand

	// IDs of the connected neighbor nodes. The neighbors are used as possible
	// targets when generating and applying the random request distribution.
	neighbors []string
}

// RunStrategy handles the execution of the strategy. It is run in a goroutine.
func (strategy *RandomStrategy) RunStrategy() error {
	logger := logging.Logger()

	logger.Info("Starting Random strategy...")

	// Initialize the pseudo-random number generator.
	seed := strategy.randomSeed
	if seed == -1 {
		seed = time.Now().UnixNano()
	}
	logger.Infof("Starting RNG with seed %d", seed)

	strategy.random = rand.New(rand.NewSource(seed))

	// FIXME: Support dynamically adding/removing functions and neighbors.
	logger.Info("Waiting 1 minute for peer and function discovery")
	time.Sleep(1 * time.Minute)

	logger.Info("Initial set-up: updating proxy with the discovered functions")
	if err := strategy.setup(); err != nil {
		return fmt.Errorf("failed to do initial set-up: %w", err)
	}

	logger.Info("Strategy started")

	ticker := time.NewTicker(_config.RecalcPeriod)
	defer ticker.Stop()

	for range ticker.C {
		start := time.Now().UTC()

		if err := strategy.recalculate(); err != nil {
			return fmt.Errorf("running random strategy: %w", err)
		}

		duration := time.Since(start)
		httpserver.StrategyIterationDuration.Set(duration.Seconds())
		httpserver.StrategySuccessIterations.Inc()

		logger.Infof("Iteration completed. Duration: %s", duration.String())
	}

	return nil
}

// OnReceived is executed every time a message from a peer is received.
func (strategy *RandomStrategy) OnReceived(msg *pubsub.Message) error {
	return nil // Unused.
}

// setup runs the initial setup of the Random strategy.
func (strategy *RandomStrategy) setup() error {
	logger := logging.Logger()

	// Get the list of deployed OpenFaaS functions.
	funcs, err := strategy.offuncsClient.GetFuncsNames()
	if err != nil {
		return fmt.Errorf("failed to check deployed functions: %w", err)
	}
	if len(funcs) == 0 {
		logger.Error("Strategy requires at least one deployed function, found none")
		return fmt.Errorf("at least one function required, found %d", len(funcs))
	}

	// Initialize the proxy configuration. After this, HAProxy weights are
	// updated through the Runtime API.
	if err := strategy.initProxyConfig(); err != nil {
		return fmt.Errorf("failed to configure proxy: %w", err)
	}

	return nil
}

// initProxyConfig initializes the proxy configuration via the Data Plane API.
//
// After this call, the proxy is managed through the Runtime API, where updates
// are performed by changing the routing weights.
func (strategy *RandomStrategy) initProxyConfig() error {
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

	// Build neighbor information.
	strategy.neighbors = nil

	neighborsPort := make(map[string]string)
	neighborsHost := make(map[string]string)

	for _, peer := range _p2pHost.Network().Peers() {
		host, err := extractSingleIPv4(_p2pHost, peer)
		if err != nil {
			return fmt.Errorf("failed to build neighbors information: %w", err)
		}

		peerID := fmt.Sprintf("node_%s", peer)

		strategy.neighbors = append(strategy.neighbors, peerID)
		neighborsHost[peerID] = host

		// FIXME: The remote proxy port may be different from local proxy port.
		neighborsPort[peerID] = strconv.FormatUint(uint64(_config.HAProxyPort), 10)
	}

	data := struct {
		Now           string
		DFaaSNodeID   string
		Functions     map[string]*uint
		Neighbors     []string
		NeighborsPort map[string]string
		NeighborsHost map[string]string
		OpenFaaSHost  string
		OpenFaaSPort  uint
		RejectorHost  string
		RejectorPort  uint
	}{
		Now:           time.Now().UTC().Format("2006-01-02 15:04:05 MST"),
		DFaaSNodeID:   _p2pHost.ID().String(),
		Functions:     funcs,
		Neighbors:     strategy.neighbors,
		NeighborsHost: neighborsHost,
		NeighborsPort: neighborsPort,
		OpenFaaSHost:  _config.OpenFaaSHost,
		OpenFaaSPort:  _config.OpenFaaSPort,
		RejectorHost:  _config.RejectorHost,
		RejectorPort:  _config.RejectorPort,
	}

	return strategy.hacfgupdater.UpdateHAConfig(data)
}

// recalculate generates a new random distribution and applies it to HAProxy.
func (strategy *RandomStrategy) recalculate() error {
	logger := logging.Logger()

	functions, err := strategy.offuncsClient.GetFuncsNames()
	if err != nil {
		return fmt.Errorf("getting function names: %w", err)
	}

	for _, function := range functions {
		backend := fmt.Sprintf("function_%s", function)

		weights := strategy.generateWeights()

		for target, weight := range weights {
			if err := strategy.runtimeapi.SetWeight(backend, target, weight); err != nil {
				return fmt.Errorf(
					"failed to set weight for %s on backend %s: %w",
					target, backend, err,
				)
			}
		}

		logger.Infof("Random distribution for %s:", function)
		for target, weight := range weights {
			logger.Infof("  target=%s weight=%d%%", target, weight)
		}
	}

	return nil
}

// generateWeights generates a random request distribution between the local
// node, the connected neighbors, and optionally the rejector.
//
// The returned weights always sum to 100.
func (strategy *RandomStrategy) generateWeights() map[string]uint {
	weights := make(map[string]uint)

	// If there are no neighbors and rejection is disabled, all requests
	// must be processed locally.
	if len(strategy.neighbors) == 0 && !strategy.randomReject {
		weights["openfaas-local"] = 100
		return weights
	}

	// Build the list of possible targets.
	targets := []string{"openfaas-local"}
	targets = append(targets, strategy.neighbors...)

	if strategy.randomReject {
		targets = append(targets, "rejector")
	}

	// Generate a random value for every target.
	values := make([]float64, len(targets))
	var total float64

	for i := range values {
		values[i] = strategy.random.Float64()
		total += values[i]
	}

	// Normalize the random values so that their sum is 100.
	var assigned uint
	for i, target := range targets {
		if i == len(targets)-1 {
			// Assign the remaining percentage to the last target so that
			// the total is exactly 100 despite rounding.
			weights[target] = 100 - assigned
			break
		}

		weight := uint(values[i] / total * 100)
		weights[target] = weight
		assigned += weight
	}

	return weights
}
