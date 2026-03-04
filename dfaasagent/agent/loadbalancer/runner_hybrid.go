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

// hybridRunner executes a HybridStrategy using a single select loop that
// handles both the periodic ticker and incoming trigger events. Tick and React
// are never called concurrently.
type hybridRunner struct {
	s       HybridStrategy
	events  chan StrategyEvent  // capacity 1; excess events dropped
	trigSet map[string]struct{}
}

func newHybridRunner(s HybridStrategy) StrategyRunner {
	trigSet := make(map[string]struct{})
	for _, t := range s.TriggerEvents() {
		trigSet[t] = struct{}{}
	}
	return &hybridRunner{
		s:       s,
		events:  make(chan StrategyEvent, 1),
		trigSet: trigSet,
	}
}

func (r *hybridRunner) Callback() communication.CBOnReceived {
	return func(msg *pubsub.Message) error {
		if err := r.s.OnReceived(msg); err != nil {
			return err
		}
		var env msgtypes.MsgEnvelope
		if err := json.Unmarshal(msg.GetData(), &env); err != nil {
			return nil
		}
		if _, ok := r.trigSet[env.Header.MsgType]; ok {
			ev := StrategyEvent{Type: env.Header.MsgType, Raw: json.RawMessage(msg.GetData())}
			select {
			case r.events <- ev:
			default: // channel full; drop
			}
		}
		return nil
	}
}

func (r *hybridRunner) Run(ctx context.Context) error {
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
				logger.Warnf("hybrid runner: Tick error (continuing): %v", err)
			}

		case ev := <-r.events:
			if err := r.s.React(ctx, ev); err != nil {
				logger.Warnf("hybrid runner: React error (continuing): %v", err)
			}
		}
	}
}
