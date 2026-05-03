// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// The loadbalancer package handles the main operational logic of the DFaaS
// agent. Using different (and selectable) strategies, it dynamically configures
// HAProxy at regular intervals and determines the node’s workload distribution.
//
// The package is implemented using a modular approach: a new strategy can be
// added by defining a factory type and a strategy type. For more details, see
// the strategyfactory.go file.
package loadbalancer

import (
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/config"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/constants"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
)

// For a running agent, only one strategy can be active at a time. These
// variables hold the necessary objects to run the strategy, following the
// singleton pattern.
var (
	_p2pHost host.Host

	// _directMessenger is used by strategies to send directed messages to peers.
	_directMessenger communication.DirectMessenger

	// Configuration given by the environment.
	_config config.Configuration

	// Strategy factory which manages the creation of the strategy instance.
	_strategyFactory strategyFactory

	// Lock used to manage the singleton strategy instance.
	_lock *sync.Mutex

	// Singleton instance representing the active strategy.
	_strategyInstance Strategy
)

// Initialize sets up the package. Warning: call this function only once!
func Initialize(p2pHost host.Host, dm communication.DirectMessenger, config config.Configuration) {
	_p2pHost = p2pHost
	_directMessenger = dm
	_config = config
	_lock = &sync.Mutex{}

	switch _config.Strategy {
	default:
		logging.Logger().Warn("No loadbalancer strategy found, using RecalcStrategy by default")
		fallthrough
	case constants.RecalcStrategy:
		_strategyFactory = &recalcStrategyFactory{}
	case constants.NodeMarginStrategy:
		_strategyFactory = &nodeMarginStrategyFactory{}
	case constants.StaticStrategy:
		_strategyFactory = &staticStrategyFactory{}
	case constants.AllLocalStrategy:
		_strategyFactory = &allLocalStrategyFactory{}
	}
}

// Strategy is the base message-receiving interface shared by all loop models.
// Every strategy must implement at least one of PeriodicStrategy,
// EventDrivenStrategy, or HybridStrategy (defined in interfaces.go).
type Strategy interface {
	// OnReceived is called for every incoming broadcast message.
	// Implementors are responsible for filtering out self-messages if needed.
	// Used for state table updates only; does NOT trigger a recalculation cycle.
	OnReceived(msg *pubsub.Message) error
}

// NewRunner creates the appropriate StrategyRunner for s based on which loop
// interface s implements. HybridStrategy takes precedence over its two
// constituents. Panics if s implements none of the known loop interfaces.
func NewRunner(s Strategy) StrategyRunner {
	switch st := s.(type) {
	case HybridStrategy:
		return newHybridRunner(st)
	case EventDrivenStrategy:
		return newEventDrivenRunner(st)
	case PeriodicStrategy:
		return newPeriodicRunner(st)
	default:
		panic("strategy implements no known loop interface (must implement PeriodicStrategy, EventDrivenStrategy, or HybridStrategy)")
	}
}

// DirectMessenger returns the DirectMessenger available to strategies for
// sending directed messages to specific peers.
func DirectMessenger() communication.DirectMessenger {
	return _directMessenger
}

// GetStrategyInstance returns the singleton Strategy instance.
func GetStrategyInstance() (Strategy, error) {
	var err error

	// In a critical section checks if the strategy is already instantiated and
	// returns it. If instance is nil, creates a new Strategy instance using the
	// createStrategy method of the strategy factory
	_lock.Lock()
	defer _lock.Unlock()

	if _strategyInstance == nil {
		_strategyInstance, err = _strategyFactory.createStrategy()
		if err != nil {
			return nil, err
		}
	}

	return _strategyInstance, nil
}
