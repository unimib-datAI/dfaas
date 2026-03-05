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

// hybridRunner executes a HybridStrategy using a single select loop that
// handles both the periodic ticker and incoming trigger events. Tick and React
// are never called concurrently.
type hybridRunner struct {
	s       HybridStrategy
	events  chan StrategyEvent  // capacity 1; excess events dropped
	trigSet map[string]struct{}
}

func newHybridRunner(s HybridStrategy) StrategyRunner {
	return &hybridRunner{
		s:       s,
		events:  make(chan StrategyEvent, 1),
		trigSet: makeTrigSet(s.TriggerEvents()),
	}
}

func (r *hybridRunner) Callback() communication.CBOnReceived {
	return makeTriggerCallback(r.s.OnReceived, r.trigSet, r.events)
}

func (r *hybridRunner) Run(ctx context.Context) error {
	logger := logging.Logger()

	ticker := time.NewTicker(effectivePeriod(r.s.Period()))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if err := r.s.Tick(ctx); err != nil {
				logger.Warnf("hybrid runner: Tick error (continuing): %v", err)
			}

		case ev := <-r.events:
			if err := r.s.React(ctx, ev); err != nil {
				logger.Warnf("hybrid runner: React error (continuing): %v", err)
			}
		}
	}
}
