// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer_test

import (
	"context"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/loadbalancer"
)

// mockPeriodic is a minimal PeriodicStrategy used in tests.
type mockPeriodic struct {
	tickCalled int
	period     time.Duration
}

func (m *mockPeriodic) OnReceived(_ *pubsub.Message) error { return nil }
func (m *mockPeriodic) Period() time.Duration              { return m.period }
func (m *mockPeriodic) Tick(_ context.Context) error       { m.tickCalled++; return nil }

// Compile-time check: mockPeriodic satisfies PeriodicStrategy.
var _ loadbalancer.PeriodicStrategy = (*mockPeriodic)(nil)

// mockEventDriven is a minimal EventDrivenStrategy used in tests.
type mockEventDriven struct {
	reactCalled int
}

func (m *mockEventDriven) OnReceived(_ *pubsub.Message) error { return nil }
func (m *mockEventDriven) TriggerEvents() []string            { return []string{"test_event"} }
func (m *mockEventDriven) Debounce() time.Duration            { return 0 }
func (m *mockEventDriven) React(_ context.Context, ev loadbalancer.StrategyEvent) error {
	m.reactCalled++
	return nil
}

var _ loadbalancer.EventDrivenStrategy = (*mockEventDriven)(nil)

// mockHybrid is a minimal HybridStrategy used in tests.
type mockHybrid struct {
	tickCalled  int
	reactCalled int
}

func (m *mockHybrid) OnReceived(_ *pubsub.Message) error { return nil }
func (m *mockHybrid) Period() time.Duration              { return 50 * time.Millisecond }
func (m *mockHybrid) Tick(_ context.Context) error       { m.tickCalled++; return nil }
func (m *mockHybrid) TriggerEvents() []string            { return []string{"test_event"} }
func (m *mockHybrid) Debounce() time.Duration            { return 0 }
func (m *mockHybrid) React(_ context.Context, ev loadbalancer.StrategyEvent) error {
	m.reactCalled++
	return nil
}

var _ loadbalancer.HybridStrategy = (*mockHybrid)(nil)
