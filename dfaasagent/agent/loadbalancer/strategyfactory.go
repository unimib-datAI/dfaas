// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	_ "embed"

	"github.com/bcicen/go-haproxy"

	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/constants"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/hacfgupd"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/forecaster"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/offuncs"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
)

// In this file is implemented the Factory creational pattern,
// useful to create the correct Strategy instance

// strategyFactory is the interface which represents a generic strategy factory.
// Every factory for new strategies must implement this interface
type strategyFactory interface {
	// Method to create a new Strategy instance
	createStrategy() (Strategy, error)
}

////////////////// RECALC STRATEGY FACTORY ///////////////////

// Struct representing the factory for Recalc strategy, which implements strategyFactory interface
type recalcStrategyFactory struct{}

//go:embed haproxycfgrecalc.tmpl
var haproxycfgrecalcTemplate string

// createStrategy creates and returns a new RecalcStrategy instance
func (strategyFactory *recalcStrategyFactory) createStrategy() (Strategy, error) {
	strategy := &RecalcStrategy{}

	strategy.nodestbl = nodestbl.NewTableRecalc(_config.RecalcPeriod + (_config.RecalcPeriod / 5))

	strategy.hacfgupdater = hacfgupd.Updater{
		HAConfigFilePath: _config.HAProxyConfigFile,
	}

	err := strategy.hacfgupdater.LoadTemplate(haproxycfgrecalcTemplate)
	if err != nil {
		return nil, err
	}

	strategy.hasockClient = haproxy.HAProxyClient{
		Addr: _config.HAProxySockPath,
	}

	strategy.offuncsClient, err = offuncs.NewClient(_config.OpenFaaSHost,
		_config.OpenFaaSPort,
		_config.OpenFaaSUser,
		_config.OpenFaaSPass)
	if err != nil {
		return nil, err
	}

	strategy.recalc = recalc{}
	strategy.it = 0

	return strategy, nil
}

////////////////// NODE MARGIN STRATEGY FACTORY ///////////////////

// Struct representing the factory for Node Margin strategy, which implements strategyFactory interface
type nodeMarginStrategyFactory struct{}

//go:embed haproxycfgnms.tmpl
var haproxycfgnmsTemplate string

// createStrategy creates and returns a new NodeMarginStrategy instance
func (strategyFactory *nodeMarginStrategyFactory) createStrategy() (Strategy, error) {
	strategy := &NodeMarginStrategy{}

	strategy.nodestbl = nodestbl.NewTableNMS(_config.RecalcPeriod * 2)

	strategy.hacfgupdater = hacfgupd.Updater{
		HAConfigFilePath: _config.HAProxyConfigFile,
	}

	err := strategy.hacfgupdater.LoadTemplate(haproxycfgnmsTemplate)
	if err != nil {
		return nil, err
	}

	strategy.offuncsClient, err = offuncs.NewClient(_config.OpenFaaSHost,
		_config.OpenFaaSPort,
		_config.OpenFaaSUser,
		_config.OpenFaaSPass)
	if err != nil {
		return nil, err
	}

	strategy.hasockClient = haproxy.HAProxyClient{
		Addr: _config.HAProxySockPath,
	}

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
