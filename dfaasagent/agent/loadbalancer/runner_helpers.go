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
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
)

// makeTrigSet converts a slice of trigger-event type strings into a set for
// O(1) membership lookups.
func makeTrigSet(triggerEvents []string) map[string]struct{} {
	set := make(map[string]struct{}, len(triggerEvents))
	for _, t := range triggerEvents {
		set[t] = struct{}{}
	}
	return set
}

// effectivePeriod returns period if non-zero, otherwise time.Minute.
func effectivePeriod(period time.Duration) time.Duration {
	if period == 0 {
		return time.Minute
	}
	return period
}

// sleepOrCtx sleeps for d or until ctx is cancelled. Returns ctx.Err() if the
// context is done, nil otherwise.
func sleepOrCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// makeTriggerCallback builds the shared pubsub callback used by both
// eventDrivenRunner and hybridRunner. It calls OnReceived for every message,
// then forwards trigger events to the runner's internal event channel.
func makeTriggerCallback(
	onReceived func(*pubsub.Message) error,
	trigSet map[string]struct{},
	events chan StrategyEvent,
) communication.CBOnReceived {
	return func(msg *pubsub.Message) error {
		if err := onReceived(msg); err != nil {
			return err
		}
		var env msgtypes.MsgEnvelope
		if err := json.Unmarshal(msg.GetData(), &env); err != nil {
			return nil
		}
		if _, ok := trigSet[env.Header.MsgType]; ok {
			ev := StrategyEvent{Type: env.Header.MsgType, Raw: json.RawMessage(msg.GetData())}
			select {
			case events <- ev:
			default: // channel full; drop — most recent event will be picked up
			}
		}
		return nil
	}
}
