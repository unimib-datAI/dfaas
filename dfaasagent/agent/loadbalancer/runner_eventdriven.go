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

// eventDrivenRunner runs an EventDrivenStrategy, calling React whenever a
// subscribed event arrives. Debounce collapses bursts of events into a single
// React call after the debounce window expires.
type eventDrivenRunner struct {
	s       EventDrivenStrategy
	events  chan StrategyEvent   // capacity 1; excess events dropped (most recent wins)
	trigSet map[string]struct{}
}

func newEventDrivenRunner(s EventDrivenStrategy) StrategyRunner {
	return &eventDrivenRunner{
		s:       s,
		events:  make(chan StrategyEvent, 1),
		trigSet: makeTrigSet(s.TriggerEvents()),
	}
}

func (r *eventDrivenRunner) Callback() communication.CBOnReceived {
	return makeTriggerCallback(r.s.OnReceived, r.trigSet, r.events)
}

func (r *eventDrivenRunner) Run(ctx context.Context) error {
	logger := logging.Logger()
	debounce := r.s.Debounce()

	var (
		pending       *StrategyEvent
		debounceTimer *time.Timer
		debounceCh    <-chan time.Time // nil when no debounce timer is active
	)

	for {
		select {
		case <-ctx.Done():
			// Pending debounced event (if any) is intentionally dropped on cancellation.
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return ctx.Err()

		case ev := <-r.events:
			if debounce == 0 {
				// No debounce: call React immediately.
				if err := r.s.React(ctx, ev); err != nil {
					logger.Warnf("event-driven runner: React error (continuing): %v", err)
				}
			} else {
				// Debounce: record latest event and reset the timer.
				pending = &ev
				if debounceTimer == nil {
					debounceTimer = time.NewTimer(debounce)
					debounceCh = debounceTimer.C
				} else {
					if !debounceTimer.Stop() {
						select {
						case <-debounceTimer.C:
						default:
						}
					}
					debounceTimer.Reset(debounce)
				}
			}

		case <-debounceCh:
			debounceTimer = nil
			debounceCh = nil
			if pending != nil {
				ev := *pending
				pending = nil
				if err := r.s.React(ctx, ev); err != nil {
					logger.Warnf("event-driven runner: React error (continuing): %v", err)
				}
			}
		}
	}
}
