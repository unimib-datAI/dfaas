// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"context"
	"time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
)

// periodicRunner runs a PeriodicStrategy on a fixed ticker.
type periodicRunner struct {
	s PeriodicStrategy
}

func newPeriodicRunner(s PeriodicStrategy) StrategyRunner {
	return &periodicRunner{s: s}
}

func (r *periodicRunner) Callback() communication.CBOnReceived {
	return r.s.OnReceived
}

func (r *periodicRunner) Run(ctx context.Context) error {
	logger := logging.Logger()

	period := r.s.Period()
	if period == 0 {
		period = time.Minute
	}

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.s.Tick(ctx); err != nil {
				logger.Warnf("periodic runner: Tick error (continuing): %v", err)
			}
		}
	}
}

// NewRunner creates the appropriate StrategyRunner for s based on which loop
// interface s implements. HybridStrategy takes precedence over its two
// constituents. Panics if s implements none of the known loop interfaces.
// TODO(Task 5): narrow to Strategy once RunStrategy() is removed from the base interface.
func NewRunner(s interface{}) StrategyRunner {
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

// newHybridRunner is a placeholder — replaced in Task 4.
// HybridStrategy also satisfies PeriodicStrategy; use that until Task 4 lands.
func newHybridRunner(s HybridStrategy) StrategyRunner {
	// NOTE: event-driven triggers are inactive until Task 4 replaces this placeholder.
	logger := logging.Logger()
	logger.Warnf("newHybridRunner: using periodic-only runner; event-driven triggers are inactive until Task 4 replaces this placeholder")
	return newPeriodicRunner(s)
}

// newEventDrivenRunner is defined in runner_eventdriven.go.
