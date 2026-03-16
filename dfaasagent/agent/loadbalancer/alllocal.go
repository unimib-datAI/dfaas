// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"context"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/httpserver"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
)

// AllLocalStrategy implements the All Local strategy, where the DFaaS node
// always processes incoming requests locally.
type AllLocalStrategy struct {
	// HAProxy client to update configuration.
	hacfgupdater hacfgupd.Updater

	// FaaS provider client to retrieve deployed functions.
	faasProvider faasprovider.FaaSProvider

	// prevFuncs holds the functions deployed at the previous tick, so we only
	// push a new HAProxy config when something changes.
	prevFuncs map[string]*uint
}

// Period returns the recalculation interval. Defaults to 1 minute if not configured.
func (strategy *AllLocalStrategy) Period() time.Duration {
	if _config.RecalcPeriod == 0 {
		return time.Minute
	}
	return _config.RecalcPeriod
}

// Tick runs one full All Local strategy decision cycle.
// The runner handles the outer loop and inter-tick sleep; Tick must not sleep
// at the end.
func (strategy *AllLocalStrategy) Tick(ctx context.Context) error {
	start := time.Now()

	funcs, err := strategy.faasProvider.GetFuncsWithTimeout()
	if err != nil {
		return fmt.Errorf("get function metadata: %w", err)
	}

	// Add 1 second to base timeout (if given) to all functions.
	for _, timeout := range funcs {
		if timeout != nil {
			*timeout += 1000
		}
	}

	// Update the configuration and reload HAProxy if changes are detected.
	equal := funcsMetadataEqual(funcs, strategy.prevFuncs) && funcsMetadataEqual(strategy.prevFuncs, funcs)
	if !equal {
		debugFuncsDiff(funcs, strategy.prevFuncs)
		logging.Logger().Info("Updating proxy due to new/deleted functions or changed timeouts")
		if err := strategy.updateProxyConfiguration(funcs); err != nil {
			return fmt.Errorf("updating proxy config: %w", err)
		}
		strategy.prevFuncs = funcs
	}

	httpserver.StrategySuccessIterations.Inc()
	httpserver.StrategyIterationDuration.Set(time.Since(start).Seconds())
	return nil
}

var _ PeriodicStrategy = (*AllLocalStrategy)(nil)

// updateProxyConfiguration updates the HAProxy configuration with the provided
// list of deployed functions. HAProxy will always be reloaded after the update.
func (strategy *AllLocalStrategy) updateProxyConfiguration(funcs map[string]*uint) error {
	// Define and populate this anonymous struct to pass data to the Go
	// template.
	data := struct {
		Now             string
		DFaaSNodeID     string
		Functions       map[string]*uint
		FaaSHost        string
		FaaSPort        uint
		FaaSBackendPath string
	}{
		Now:             time.Now().Format("2006-01-02 15:04:05"),
		DFaaSNodeID:     _p2pHost.ID().String(),
		Functions:       funcs,
		FaaSHost:        _config.FaaSHost,
		FaaSPort:        _config.FaaSPort,
		FaaSBackendPath: faasprovider.BackendPathPrefix(_config.FaaSPlatform, _config.OpenWhiskNamespace),
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
