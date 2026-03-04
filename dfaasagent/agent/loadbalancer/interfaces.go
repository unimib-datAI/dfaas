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
)

// PeriodicStrategy executes Tick on a fixed schedule managed by the runner.
type PeriodicStrategy interface {
	// OnReceived is called for every incoming broadcast message.
	// Used for state table updates; does NOT trigger a recalculation cycle.
	// Implementors are responsible for filtering out self-messages if needed.
	OnReceived(msg *pubsub.Message) error

	// Period returns the interval between consecutive Tick calls.
	// If zero, the runner defaults to 1 minute.
	Period() time.Duration

	// Tick runs one full decision cycle: collect data, compute weights, update HAProxy.
	// The runner serialises calls — Tick is never called concurrently with itself.
	// A non-nil error is logged but does not stop the loop.
	Tick(ctx context.Context) error
}

// EventDrivenStrategy executes React whenever a subscribed event arrives.
type EventDrivenStrategy interface {
	// OnReceived is called for every incoming broadcast message.
	// Used for state table updates; does NOT trigger React.
	// Implementors are responsible for filtering out self-messages if needed.
	OnReceived(msg *pubsub.Message) error

	// TriggerEvents returns the msgtypes.Type* constants that cause React to be invoked.
	TriggerEvents() []string

	// Debounce returns the minimum interval between consecutive React calls.
	// 0 means no debounce (React is called immediately for each trigger event).
	Debounce() time.Duration

	// React runs one full decision cycle in response to ev.
	// A non-nil error is logged but does not stop the loop.
	React(ctx context.Context, ev StrategyEvent) error
}

// HybridStrategy combines a periodic baseline with event-driven triggers.
// The runner guarantees that Tick and React are never called concurrently.
// Both embedded interfaces declare OnReceived with an identical signature;
// Go merges them into one — implementors provide a single OnReceived method.
type HybridStrategy interface {
	PeriodicStrategy
	EventDrivenStrategy
}

// StrategyEvent carries the type discriminator and raw JSON payload of the
// broadcast message that triggered a React call.
type StrategyEvent struct {
	// Type is one of the msgtypes.Type* constants.
	Type string
	// Raw is valid JSON; callers may unmarshal it into their concrete message type.
	Raw json.RawMessage
}

// StrategyRunner manages the execution lifecycle of a strategy.
type StrategyRunner interface {
	// Callback returns the pubsub CBOnReceived to register with the receiver.
	// For Periodic strategies it simply delegates to OnReceived.
	// For EventDriven/Hybrid strategies it also forwards trigger events to the
	// internal event channel.
	// Must be called before Run.
	Callback() communication.CBOnReceived

	// Run blocks, executing the strategy loop until ctx is cancelled or a fatal
	// error occurs.
	Run(ctx context.Context) error
}
