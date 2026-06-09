// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/httpserver"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
)

// AllLocalStrategy implements the All Local strategy, where the DFaaS node
// always processes incoming requests locally.
type AllLocalStrategy struct {
	// HAProxy client to update configuration.
	hacfgupdater *hacfgupd.Updater

	// OpenFaaS Gateway client to retrieve deployed functions.
	offuncsClient *offuncs.Client

	// AllLocalStrategy is an iteration-based strategy, where a method is
	// executed every N seconds to check the current deployment state. If the
	// latest deployment differs from the expected state, the proxy
	// configuration is updated accordingly. This is why we need prevFuncs: it
	// stores metadata of the latest deployment to detect changes between
	// iterations.
	prevFuncs map[string]*uint
}

// RunStrategy handles the periodic execution of the strategy's iteration
// function. It should run in a goroutine.
func (strategy *AllLocalStrategy) RunStrategy() error {
	logger := logging.Logger()
	logger.Debug("Starting All Local strategy...")

	// prevFuncs starts as empty since no deploy has been done.
	strategy.prevFuncs = make(map[string]*uint)

	// Avoid waiting the first ticker's iteration.
	if err := strategy.runIteration(); err != nil {
		return fmt.Errorf("running first strategy iteration: %w", err)
	}

	ticker := time.NewTicker(_config.RecalcPeriod)
	defer ticker.Stop()
	for range ticker.C {
		if err := strategy.runIteration(); err != nil {
			return fmt.Errorf("running strategy iteration: %w", err)
		}
	}

	return nil
}

// runIteration runs a single iteration of the strategy: it updates the HAProxy
// configuration when the list of functions changes (or the first time), and
// nothing more.
func (strategy *AllLocalStrategy) runIteration() error {
	logger := logging.Logger()

	start := time.Now().UTC()

	// GetFuncsWithTimeout always returns a new map.
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

	// Update the configuration and reload HAProxy if changes are detected.
	equal := funcsMetadataEqual(funcs, strategy.prevFuncs) && funcsMetadataEqual(strategy.prevFuncs, funcs)
	if !equal {
		debugFuncsDiff(funcs, strategy.prevFuncs)
		logger.Info("Updating proxy due to new/deleted functions or changed timeouts")
		if err = strategy.updateProxyConfiguration(funcs); err != nil {
			return fmt.Errorf("updating proxy config: %w", err)
		}
		strategy.prevFuncs = funcs
	}

	duration := time.Since(start)
	httpserver.StrategySuccessIterations.Inc()
	httpserver.StrategyIterationDuration.Set(duration.Seconds())
	logger.Infof("Iteration completed. Duration: %s", duration.String())

	return nil
}

// updateProxyConfiguration updates the HAProxy configuration with the provided
// list of deployed functions. HAProxy will always be reloaded after the update.
func (strategy *AllLocalStrategy) updateProxyConfiguration(funcs map[string]*uint) error {
	// Define and populate this anonymous struct to pass data to the Go
	// template.
	data := struct {
		Now          string
		DFaaSNodeID  string
		Functions    map[string]*uint
		OpenFaaSHost string
		OpenFaaSPort uint
	}{
		Now:          time.Now().UTC().Format("2006-01-02 15:04:05 MST"),
		DFaaSNodeID:  _p2pHost.ID().String(),
		Functions:    funcs,
		OpenFaaSHost: _config.OpenFaaSHost,
		OpenFaaSPort: _config.OpenFaaSPort,
	}

	return strategy.hacfgupdater.UpdateHAConfig(data)
}

// OnReceived is executed every time a message from a peer is received. In this
// strategy we simply ignore all messages.
func (strategy *AllLocalStrategy) OnReceived(msg *pubsub.Message) error {
	return nil
}

// funcsMetadataEqual returns true if the given a and b maps are equal. This is
// not a symmetric operation.
func funcsMetadataEqual(a, b map[string]*uint) bool {
	if len(a) != len(b) {
		return false
	}

	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}

		if av == nil && bv == nil {
			continue
		}

		if av == nil || bv == nil {
			return false
		}

		if *av != *bv {
			return false
		}
	}
	return true
}
