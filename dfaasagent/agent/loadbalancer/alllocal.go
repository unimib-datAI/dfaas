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
	hacfgupdater hacfgupd.Updater

	// OpenFaaS Gateway client to retrive deployed functions.
	offuncsClient *offuncs.Client
}

// RunStrategy handles the periodic execution of the recalculation function. It
// should run in a goroutine.
func (strategy *AllLocalStrategy) RunStrategy() error {
	logger := logging.Logger()

	var millisNow, millisSleep int64

	// Functions deployed in OpenFaaS at the previous cycle. At start is empty.
	prevFuncs := make(map[string]*uint)

	millisInterval := int64(_config.RecalcPeriod / time.Millisecond)

	// This strategy is straightforward: we only need to update the HAProxy
	// configuration when the list of functions changes, and nothing more.
	for {
		start := time.Now()

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
		equal := funcsMetadataEqual(funcs, prevFuncs) && funcsMetadataEqual(prevFuncs, funcs)
		if !equal {
			debugFuncsDiff(funcs, prevFuncs)
			logger.Info("Updating proxy due to new/deleted functions or changed timeouts")
			if err = strategy.updateProxyConfiguration(funcs); err != nil {
				return fmt.Errorf("updating proxy config: %w", err)
			}
			prevFuncs = funcs
		}

		duration := time.Since(start)

		// Metrics exposed to Prometheus.
		httpserver.StrategySuccessIterations.Inc()
		httpserver.StrategyIterationDuration.Set(duration.Seconds())

		// Suspend the goroutine until the start of the next cycle/period.
		// Aligns the next iteration with the fixed periodic "ticks" of
		// millisInterval.
		millisNow = time.Now().UnixNano() / 1000000
		millisSleep = millisInterval - (millisNow % millisInterval)
		time.Sleep(time.Duration(millisSleep) * time.Millisecond)
	}
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
		Now:          time.Now().Format("2006-01-02 15:04:05"),
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

// funcsMetadataEqual returns true if the given a and b maps are equal.
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
