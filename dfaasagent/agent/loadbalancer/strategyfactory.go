// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	_ "embed"
	"fmt"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/constants"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/forecaster"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

// This file contains the implementation of the factory pattern for defining and
// creating specific load balancing strategies.
//
// The core of the factory is the strategyFactor interface. To create a new
// strategy, you need to define two types:
//
// 1. One that implements the strategyFactor interface,
// 2. One that implements the Strategy interface.
//
// Usually, each strategy has a corresponding HAProxy configuration template,
// embedded using the embed module.
//
// After defining a new strategy factory and strategy, you need to update the
// registry in the Initialize method in the loadbalancer.go filep.

// strategyFactory is the interface which represents a generic strategy factory.
// Every factory for new strategies must implement this interface.
type strategyFactory interface {
	// Creates and returns a new Strategy instance for this strategy.
	createStrategy() (Strategy, error)
}

// recalcStrategyFactory is the strategy factory for the Recalc strategy.
type recalcStrategyFactory struct{}

//go:embed haproxycfgrecalc.tmpl
var haproxycfgrecalcTemplate string

// createStrategy creates and returns a new RecalcStrategy instance.
func (strategyFactory *recalcStrategyFactory) createStrategy() (Strategy, error) {
	strategy := &RecalcStrategy{}

	// Avoid premature expiration of nodes in the table between strategy runs.
	strategy.nodestbl = nodestbl.NewTableRecalc(_config.RecalcPeriod + (_config.RecalcPeriod / 5))

	strategy.hacfgupdater = hacfgupd.Updater{}
	if err := strategy.hacfgupdater.LoadTemplate(haproxycfgrecalcTemplate); err != nil {
		return nil, fmt.Errorf("loading HAProxy config. template: %w", err)
	}

	strategy.offuncsClient = offuncs.NewClient(_config.OpenFaaSHost, _config.OpenFaaSPort)

	strategy.it = 0

	return strategy, nil
}

// nodeStrategyFactory is the strategy factory for the Node Margin strategy.
type nodeMarginStrategyFactory struct{}

//go:embed haproxycfgnms.tmpl
var haproxycfgnmsTemplate string

// createStrategy creates and returns a new NodeMarginStrategy instance.
func (strategyFactory *nodeMarginStrategyFactory) createStrategy() (Strategy, error) {
	strategy := &NodeMarginStrategy{}

	strategy.nodestbl = nodestbl.NewTableNMS(_config.RecalcPeriod * 2)

	strategy.hacfgupdater = hacfgupd.Updater{}

	err := strategy.hacfgupdater.LoadTemplate(haproxycfgnmsTemplate)
	if err != nil {
		return nil, err
	}

	strategy.offuncsClient = offuncs.NewClient(_config.OpenFaaSHost, _config.OpenFaaSPort)

	strategy.forecasterClient = forecaster.Client{
		Hostname: constants.ForecasterHost,
		Port:     constants.ForeasterPort,
	}

	strategy.nodeInfo = nodeInfo{}
	strategy.maxValues = make(map[string]float64)
	strategy.targetNodes = make(map[string][]string)
	strategy.weights = make(map[string]map[string]uint)

	return strategy, nil
}

// staticStrategyFactory is the strategy factory for the Static strategy.
type staticStrategyFactory struct{}

//go:embed haproxycfgstatic.tmpl
var haproxycfgStaticTemplate string

// createStrategy creates and returns a new Static instance.
func (strategyFactory *staticStrategyFactory) createStrategy() (Strategy, error) {
	strategy := &StaticStrategy{}

	// TODO: Use a custom table.
	strategy.nodestbl = nodestbl.NewTableNMS(_config.RecalcPeriod * 2)

	strategy.hacfgupdater = hacfgupd.Updater{}

	if err := strategy.hacfgupdater.LoadTemplate(haproxycfgStaticTemplate); err != nil {
		return nil, fmt.Errorf("loading HAProxy config. template: %w", err)
	}

	strategy.offuncsClient = offuncs.NewClient(_config.OpenFaaSHost, _config.OpenFaaSPort)

	strategy.nodeInfo = nodeInfoStatic{}
	strategy.targetNodes = make(map[string][]string)
	strategy.weights = make(map[string]map[string]uint)

	return strategy, nil
}

// allLocalStrategyFactory is the strategy factory for the All Local strategy.
type allLocalStrategyFactory struct{}

//go:embed haproxycfgalllocal.tmpl
var haproxycfgAllLocalTemplate string

// createStrategy creates and returns a new All Local instance.
func (strategyFactory *allLocalStrategyFactory) createStrategy() (Strategy, error) {
	strategy := &AllLocalStrategy{}

	strategy.hacfgupdater = hacfgupd.Updater{}

	if err := strategy.hacfgupdater.LoadTemplate(haproxycfgAllLocalTemplate); err != nil {
		return nil, fmt.Errorf("loading HAProxy config. template: %w", err)
	}

	strategy.offuncsClient = offuncs.NewClient(_config.OpenFaaSHost, _config.OpenFaaSPort)

	return strategy, nil
}
