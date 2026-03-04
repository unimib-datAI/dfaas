// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"context"
	"encoding/json"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
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
	trigSet := make(map[string]struct{})
	for _, t := range s.TriggerEvents() {
		trigSet[t] = struct{}{}
	}
	return &eventDrivenRunner{
		s:       s,
		events:  make(chan StrategyEvent, 1),
		trigSet: trigSet,
	}
}

func (r *eventDrivenRunner) Callback() communication.CBOnReceived {
	return func(msg *pubsub.Message) error {
		if err := r.s.OnReceived(msg); err != nil {
			return err
		}
		// Forward trigger events to the worker goroutine.
		var env msgtypes.MsgEnvelope
		if err := json.Unmarshal(msg.GetData(), &env); err != nil {
			return nil
		}
		if _, ok := r.trigSet[env.Header.MsgType]; ok {
			ev := StrategyEvent{Type: env.Header.MsgType, Raw: json.RawMessage(msg.GetData())}
			select {
			case r.events <- ev:
			default: // channel full; drop — most recent event will be picked up
			}
		}
		return nil
	}
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
