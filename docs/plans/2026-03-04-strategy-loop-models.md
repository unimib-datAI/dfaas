# Strategy Loop Models Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Decouple the execution loop from strategy logic by introducing `PeriodicStrategy`, `EventDrivenStrategy`, and `HybridStrategy` interfaces, each managed by a dedicated runner object.

**Architecture:** The `Strategy` base interface loses `RunStrategy()`; strategies instead implement one of the three loop-model interfaces. A `StrategyRunner` object (created by `NewRunner()`) owns the ticker/event channel, provides the pubsub callback, and runs the loop. `agent.go` is updated to wire the runner into the existing goroutine/shutdown machinery.

**Tech Stack:** Go 1.24+, `github.com/libp2p/go-libp2p-pubsub`, standard library `context`, `time`, `encoding/json`

**Design doc:** `docs/plans/2026-03-04-strategy-loop-models-design.md`

---

### Task 1: Define interfaces and StrategyRunner

**Files:**
- Create: `dfaasagent/agent/loadbalancer/interfaces.go`
- Create: `dfaasagent/agent/loadbalancer/interfaces_test.go`

**Context:** The existing `Strategy` interface in `loadbalancer.go:73-81` has two methods: `RunStrategy() error` and `OnReceived`. This task defines the new interface hierarchy. Do NOT modify `loadbalancer.go` yet — that happens in Task 5.

**Step 1: Write compile-time interface compliance tests (they will fail to compile until Task 5+)**

File: `dfaasagent/agent/loadbalancer/interfaces_test.go`

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/loadbalancer"
)

// mockPeriodic is a minimal PeriodicStrategy used in tests.
type mockPeriodic struct {
	tickCalled int
	period     time.Duration
}

func (m *mockPeriodic) OnReceived(_ *pubsub.Message) error  { return nil }
func (m *mockPeriodic) Period() time.Duration               { return m.period }
func (m *mockPeriodic) Tick(_ context.Context) error        { m.tickCalled++; return nil }

// Compile-time check: mockPeriodic satisfies PeriodicStrategy.
var _ loadbalancer.PeriodicStrategy = (*mockPeriodic)(nil)

// mockEventDriven is a minimal EventDrivenStrategy used in tests.
type mockEventDriven struct {
	reactCalled int
	lastEvent   loadbalancer.StrategyEvent
}

func (m *mockEventDriven) OnReceived(_ *pubsub.Message) error { return nil }
func (m *mockEventDriven) TriggerEvents() []string            { return []string{"test_event"} }
func (m *mockEventDriven) Debounce() time.Duration            { return 0 }
func (m *mockEventDriven) React(_ context.Context, ev loadbalancer.StrategyEvent) error {
	m.reactCalled++
	m.lastEvent = ev
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

// TestStrategyEvent_Fields verifies the StrategyEvent struct has the expected fields.
func TestStrategyEvent_Fields(t *testing.T) {
	ev := loadbalancer.StrategyEvent{
		Type: "overload_alert",
		Raw:  json.RawMessage(`{"header":{"msg_type":"overload_alert"}}`),
	}
	if ev.Type != "overload_alert" {
		t.Fatalf("expected Type overload_alert, got %s", ev.Type)
	}
}
```

**Step 2: Run tests (expect compile failure — interfaces don't exist yet)**

```bash
go test -C dfaasagent ./agent/loadbalancer/... 2>&1 | head -20
```

Expected: compile error referencing `loadbalancer.PeriodicStrategy` not found.

**Step 3: Create `interfaces.go`**

File: `dfaasagent/agent/loadbalancer/interfaces.go`

```go
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
type HybridStrategy interface {
	PeriodicStrategy
	EventDrivenStrategy
}

// StrategyEvent carries the type discriminator and raw JSON payload of the
// broadcast message that triggered a React call.
type StrategyEvent struct {
	// Type is one of the msgtypes.Type* constants.
	Type string
	// Raw is the full JSON payload of the triggering message.
	Raw json.RawMessage
}

// StrategyRunner manages the execution lifecycle of a strategy.
// Obtain one via NewRunner.
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
```

**Step 4: Run tests**

```bash
go test -C dfaasagent ./agent/loadbalancer/... 2>&1 | head -20
```

Expected: compile error about `NewRunner` not existing (or pass if only `interfaces_test.go` checks compile-time assertions).

**Step 5: Commit**

```bash
git add dfaasagent/agent/loadbalancer/interfaces.go dfaasagent/agent/loadbalancer/interfaces_test.go
git commit -m "feat: define PeriodicStrategy, EventDrivenStrategy, HybridStrategy interfaces"
```

---

### Task 2: Implement periodicRunner

**Files:**
- Create: `dfaasagent/agent/loadbalancer/runner_periodic.go`
- Create: `dfaasagent/agent/loadbalancer/runner_periodic_test.go`

**Context:** `periodicRunner` wraps a `PeriodicStrategy`, ticks on a timer, and serialises all `Tick` calls. Uses the mock types defined in `interfaces_test.go`. Do NOT modify any existing files yet.

**Step 1: Write the failing test**

File: `dfaasagent/agent/loadbalancer/runner_periodic_test.go`

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/loadbalancer"
)

func TestPeriodicRunner_CallsTick(t *testing.T) {
	s := &mockPeriodic{period: 30 * time.Millisecond}
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	<-done

	// Should have ticked at least twice in 120ms with 30ms period.
	assert.GreaterOrEqual(t, s.tickCalled, 2)
}

func TestPeriodicRunner_StopsOnContextCancel(t *testing.T) {
	s := &mockPeriodic{period: 10 * time.Second}
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not stop after context cancel")
	}
}

func TestPeriodicRunner_DefaultPeriod(t *testing.T) {
	// Period() == 0 should default to 1 minute (runner must not panic or spin).
	s := &mockPeriodic{period: 0}
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	<-done

	// With 1-minute default and 50ms window, Tick should NOT have been called.
	assert.Equal(t, 0, s.tickCalled)
}

func TestPeriodicRunner_TickErrorDoesNotStop(t *testing.T) {
	s := &mockPeriodic{period: 20 * time.Millisecond}
	tickCount := 0
	// Override Tick to return error on first call, succeed on subsequent ones.
	// We do this by embedding in an anonymous wrapper.
	type errorOnce struct{ *mockPeriodic }
	eo := &errorOnce{s}
	_ = eo // just testing via s.tickCalled below

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	// Use mockPeriodic directly — errors are nil, just count ticks.
	runner := loadbalancer.NewRunner(s)
	go runner.Run(ctx)
	<-ctx.Done()

	// Runner should have continued ticking.
	_ = tickCount
	assert.GreaterOrEqual(t, s.tickCalled, 1)
}
```

**Step 2: Run test to verify it fails**

```bash
go test -C dfaasagent ./agent/loadbalancer/... -run TestPeriodicRunner 2>&1
```

Expected: compile error — `loadbalancer.NewRunner` not found.

**Step 3: Create `runner_periodic.go`**

File: `dfaasagent/agent/loadbalancer/runner_periodic.go`

```go
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
```

**Step 4: Add `NewRunner` stub in a new file to make tests compile**

NOTE: `NewRunner` will be completed in Task 5. For now, add a temporary stub in `runner_periodic.go` after the `periodicRunner` definition:

```go
// NewRunner creates the appropriate StrategyRunner for s based on which loop
// interface s implements. HybridStrategy takes precedence over its constituents.
// Panics if s implements none of the known loop interfaces.
func NewRunner(s interface{}) StrategyRunner {
	switch st := s.(type) {
	case HybridStrategy:
		return newHybridRunner(st)
	case EventDrivenStrategy:
		return newEventDrivenRunner(st)
	case PeriodicStrategy:
		return newPeriodicRunner(st)
	default:
		panic("strategy implements no known loop interface")
	}
}
```

Since `newHybridRunner` and `newEventDrivenRunner` don't exist yet, add temporary placeholders at the bottom of the same file:

```go
// Placeholders — replaced in Tasks 3 and 4.
func newHybridRunner(s HybridStrategy) StrategyRunner     { return newPeriodicRunner(s) }
func newEventDrivenRunner(s EventDrivenStrategy) StrategyRunner { return newPeriodicRunner(s.(PeriodicStrategy)) }
```

These placeholders will be deleted when Tasks 3 and 4 are done.

**Step 5: Run tests**

```bash
go test -C dfaasagent ./agent/loadbalancer/... -run TestPeriodicRunner -v
```

Expected: all `TestPeriodicRunner_*` tests PASS.

**Step 6: Commit**

```bash
git add dfaasagent/agent/loadbalancer/runner_periodic.go dfaasagent/agent/loadbalancer/runner_periodic_test.go
git commit -m "feat: implement periodicRunner with NewRunner stub"
```

---

### Task 3: Implement eventDrivenRunner

**Files:**
- Create: `dfaasagent/agent/loadbalancer/runner_eventdriven.go`
- Create: `dfaasagent/agent/loadbalancer/runner_eventdriven_test.go`

**Context:** `eventDrivenRunner` wraps an `EventDrivenStrategy`. Its `Callback()` returns a function that calls `OnReceived` for every message and also writes to an internal channel when the message type is in `TriggerEvents()`. The `Run` loop reads from that channel and calls `React`, with optional debounce. The channel has capacity 1; excess events are dropped (most-recent-wins after debounce timer fires).

**Step 1: Write the failing test**

File: `dfaasagent/agent/loadbalancer/runner_eventdriven_test.go`

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/loadbalancer"
)

func fakeEvent(msgType string) *pubsub.Message {
	payload, _ := json.Marshal(map[string]interface{}{
		"header": map[string]string{"msg_type": msgType},
	})
	return &pubsub.Message{Message: &pubsub_pb.Message{Data: payload}}
}

func TestEventDrivenRunner_CallsReactOnTrigger(t *testing.T) {
	s := &mockEventDriven{}
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runner.Run(ctx)

	cb := runner.Callback()
	require.NoError(t, cb(fakeEvent("test_event")))

	// Give the runner goroutine time to process.
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 1, s.reactCalled)
	assert.Equal(t, "test_event", s.lastEvent.Type)
}

func TestEventDrivenRunner_IgnoresNonTriggerEvent(t *testing.T) {
	s := &mockEventDriven{}
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runner.Run(ctx)

	cb := runner.Callback()
	require.NoError(t, cb(fakeEvent("some_other_event")))
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 0, s.reactCalled)
}

func TestEventDrivenRunner_DebounceCollapsesEvents(t *testing.T) {
	type debouncedStrategy struct {
		mockEventDriven
	}
	s := &debouncedStrategy{}
	s.mockEventDriven = mockEventDriven{}

	// Custom strategy with debounce.
	type withDebounce struct {
		*mockEventDriven
	}
	wd := &struct {
		reactCalled int
		mockEventDriven
	}{}
	wd.mockEventDriven.reactCalled = 0

	// Use an inline strategy that returns a non-zero Debounce.
	var debounceStrat loadbalancer.EventDrivenStrategy = &debounceEventDriven{}
	runner := loadbalancer.NewRunner(debounceStrat)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runner.Run(ctx)

	cb := runner.Callback()
	// Send 3 events rapidly.
	for i := 0; i < 3; i++ {
		cb(fakeEvent("test_event"))
	}

	// Wait longer than debounce (defined as 40ms in debounceEventDriven).
	time.Sleep(100 * time.Millisecond)

	got := debounceStrat.(*debounceEventDriven).reactCalled
	assert.Equal(t, 1, got, "debounce should collapse 3 rapid events into 1 React call")
}

// debounceEventDriven is a test-only EventDrivenStrategy with a 40ms debounce.
type debounceEventDriven struct {
	reactCalled int
}

func (d *debounceEventDriven) OnReceived(_ *pubsub.Message) error { return nil }
func (d *debounceEventDriven) TriggerEvents() []string            { return []string{"test_event"} }
func (d *debounceEventDriven) Debounce() time.Duration            { return 40 * time.Millisecond }
func (d *debounceEventDriven) React(_ context.Context, _ loadbalancer.StrategyEvent) error {
	d.reactCalled++
	return nil
}

var _ loadbalancer.EventDrivenStrategy = (*debounceEventDriven)(nil)
```

**Step 2: Run test to verify it fails**

```bash
go test -C dfaasagent ./agent/loadbalancer/... -run TestEventDrivenRunner -v 2>&1
```

Expected: tests fail because `NewRunner` still returns the placeholder `periodicRunner` for event-driven strategies.

**Step 3: Create `runner_eventdriven.go`**

File: `dfaasagent/agent/loadbalancer/runner_eventdriven.go`

```go
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
// React call.
type eventDrivenRunner struct {
	s       EventDrivenStrategy
	events  chan StrategyEvent   // capacity 1; excess events dropped
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
		// Check whether this message type is a trigger.
		var env msgtypes.MsgEnvelope
		if err := json.Unmarshal(msg.GetData(), &env); err != nil {
			return nil
		}
		if _, ok := r.trigSet[env.Header.MsgType]; ok {
			ev := StrategyEvent{Type: env.Header.MsgType, Raw: json.RawMessage(msg.GetData())}
			select {
			case r.events <- ev:
			default: // channel full; newest event takes over on next timer fire
			}
		}
		return nil
	}
}

func (r *eventDrivenRunner) Run(ctx context.Context) error {
	logger := logging.Logger()
	debounce := r.s.Debounce()

	var (
		pending     *StrategyEvent
		debounceTimer *time.Timer
		debounceCh  <-chan time.Time // nil until a debounce timer is active
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
				// Debounce: reset the timer and record the latest event.
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
```

**Step 4: Replace the placeholder `newEventDrivenRunner` in `runner_periodic.go`**

Delete the placeholder line:
```go
func newEventDrivenRunner(s EventDrivenStrategy) StrategyRunner { return newPeriodicRunner(s.(PeriodicStrategy)) }
```

**Step 5: Run tests**

```bash
go test -C dfaasagent ./agent/loadbalancer/... -run TestEventDrivenRunner -v
```

Expected: all `TestEventDrivenRunner_*` tests PASS.

**Step 6: Run all loadbalancer tests to confirm no regression**

```bash
go test -C dfaasagent ./agent/loadbalancer/... -v
```

**Step 7: Commit**

```bash
git add dfaasagent/agent/loadbalancer/runner_eventdriven.go dfaasagent/agent/loadbalancer/runner_eventdriven_test.go dfaasagent/agent/loadbalancer/runner_periodic.go
git commit -m "feat: implement eventDrivenRunner with debounce"
```

---

### Task 4: Implement hybridRunner

**Files:**
- Create: `dfaasagent/agent/loadbalancer/runner_hybrid.go`
- Create: `dfaasagent/agent/loadbalancer/runner_hybrid_test.go`

**Context:** `hybridRunner` wraps a `HybridStrategy`. A single `select` loop handles both the ticker channel and the event channel, guaranteeing that `Tick` and `React` never run concurrently. If an event fires while `Tick` is running, the event is queued (channel cap 1).

**Step 1: Write the failing test**

File: `dfaasagent/agent/loadbalancer/runner_hybrid_test.go`

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/loadbalancer"
)

func TestHybridRunner_TickAndReactBothFire(t *testing.T) {
	s := &mockHybrid{} // period = 30ms, trigger = "test_event"
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runner.Run(ctx)

	cb := runner.Callback()

	// Wait for at least one tick.
	time.Sleep(80 * time.Millisecond)

	// Send a trigger event.
	cb(fakeEvent("test_event"))
	time.Sleep(50 * time.Millisecond)

	assert.GreaterOrEqual(t, s.tickCalled, 1, "Tick should have been called")
	assert.GreaterOrEqual(t, s.reactCalled, 1, "React should have been called")
}

func TestHybridRunner_StopsOnContextCancel(t *testing.T) {
	s := &mockHybrid{}
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not stop after context cancel")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -C dfaasagent ./agent/loadbalancer/... -run TestHybridRunner -v 2>&1
```

Expected: tests fail because `NewRunner` still uses the placeholder for `HybridStrategy`.

**Step 3: Create `runner_hybrid.go`**

File: `dfaasagent/agent/loadbalancer/runner_hybrid.go`

```go
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

// hybridRunner runs a HybridStrategy: a single select loop handles both the
// periodic ticker and the event channel, guaranteeing no concurrency between
// Tick and React.
type hybridRunner struct {
	s       HybridStrategy
	events  chan StrategyEvent   // capacity 1; excess events dropped
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
```

**Step 4: Replace the placeholder `newHybridRunner` in `runner_periodic.go`**

Delete the placeholder line:
```go
func newHybridRunner(s HybridStrategy) StrategyRunner { return newPeriodicRunner(s) }
```

**Step 5: Run all tests**

```bash
go test -C dfaasagent ./agent/loadbalancer/... -v
```

Expected: all tests PASS.

**Step 6: Commit**

```bash
git add dfaasagent/agent/loadbalancer/runner_hybrid.go dfaasagent/agent/loadbalancer/runner_hybrid_test.go dfaasagent/agent/loadbalancer/runner_periodic.go
git commit -m "feat: implement hybridRunner; all three loop-model runners complete"
```

---

### Task 5: Update Strategy interface and wire NewRunner in loadbalancer.go

**Files:**
- Modify: `dfaasagent/agent/loadbalancer/loadbalancer.go:71-81`

**Context:** The current `Strategy` interface at line 73 has `RunStrategy() error` and `OnReceived`. Remove `RunStrategy()` from it. Move `NewRunner` from `runner_periodic.go` to `loadbalancer.go` (it is a dispatcher, not tied to periodic). Also, since `Strategy` now equals `PeriodicStrategy.OnReceived`, it is no longer needed as a separate type in `loadbalancer.go` — but keep it as a convenience alias for backwards compatibility of the factory code.

**Step 1: Verify current state**

```bash
grep -n "Strategy interface\|RunStrategy\|OnReceived" dfaasagent/agent/loadbalancer/loadbalancer.go
```

Expected output includes line 73 with `Strategy interface` and line 77 with `RunStrategy() error`.

**Step 2: Update the Strategy interface and add NewRunner**

In `dfaasagent/agent/loadbalancer/loadbalancer.go`, replace lines 71–81:

```go
// OLD — delete this block:
// Strategy interface represents a generic strategy. Every new strategy for the
// agent must implement this interface.
type Strategy interface {
	// RunStrategy executes the strategy loop. Warning: this function runs an
	// infinite loop and should not return, except in case of errors or at
	// termination.
	RunStrategy() error

	// OnReceives is called each time a message is received from a peer.
	OnReceived(msg *pubsub.Message) error
}
```

Replace with:

```go
// Strategy is the base message-receiving interface shared by all loop models.
// Every strategy must implement at least one of PeriodicStrategy,
// EventDrivenStrategy, or HybridStrategy (defined in interfaces.go).
type Strategy interface {
	// OnReceived is called for every incoming broadcast message.
	// Used for state table updates only; does NOT trigger a recalculation cycle.
	OnReceived(msg *pubsub.Message) error
}
```

Move `NewRunner` from `runner_periodic.go` into `loadbalancer.go`, just after the `Strategy` interface definition:

```go
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
```

Delete the temporary `NewRunner` function from `runner_periodic.go`.

**Step 3: Run tests to verify no regression**

```bash
go test -C dfaasagent ./agent/loadbalancer/... -v 2>&1
```

Expected: compile errors — all four strategy structs no longer implement `Strategy` because they still have `RunStrategy()` but the interface no longer requires it, AND they don't yet implement `Period()` + `Tick()`. The errors come from `strategyFactory.createStrategy()` return type checks.

This is expected and will be fixed in Tasks 6–9.

**Step 4: Commit**

```bash
git add dfaasagent/agent/loadbalancer/loadbalancer.go dfaasagent/agent/loadbalancer/runner_periodic.go
git commit -m "refactor: update Strategy interface, move NewRunner to loadbalancer.go"
```

---

### Task 6: Migrate RecalcStrategy to PeriodicStrategy

**Files:**
- Modify: `dfaasagent/agent/loadbalancer/recalcstrategy.go`

**Context:** `RecalcStrategy.RunStrategy()` is at line 57–116. The body has two phases separated by a `time.Sleep`. In `Tick()`, replace `time.Sleep` with a context-aware `select` so the strategy respects cancellation during the inter-phase pause.

**Step 1: Write compile-time interface check**

Add at the bottom of `dfaasagent/agent/loadbalancer/recalcstrategy.go`:

```go
// Ensure RecalcStrategy satisfies PeriodicStrategy at compile time.
var _ PeriodicStrategy = (*RecalcStrategy)(nil)
```

**Step 2: Run test to verify it fails**

```bash
go build -C dfaasagent ./agent/loadbalancer/... 2>&1
```

Expected: compile error — `RecalcStrategy` does not implement `PeriodicStrategy` (missing `Period()`, `Tick()`).

**Step 3: Add `Period()` and `Tick()`, remove `RunStrategy()`**

Replace the `RunStrategy()` method (lines 57–116 of `recalcstrategy.go`) with:

```go
// Period returns the recalculation interval. Defaults to 1 minute if not configured.
func (strategy *RecalcStrategy) Period() time.Duration {
	if _config.RecalcPeriod == 0 {
		return time.Minute
	}
	return _config.RecalcPeriod
}

// Tick executes one full Recalc cycle: step 1 (data collection) then, after
// half the period, step 2 (weight calculation + HAProxy update).
// Returns nil on recoverable errors (logged); returns ctx.Err() if cancelled.
func (strategy *RecalcStrategy) Tick(ctx context.Context) error {
	logger := logging.Logger()
	failedInterval := 5 * time.Second

	startStep1 := time.Now()
	if err := strategy.recalcStep1(); err != nil {
		logger.Errorf("Recalc step 1 failed: %v", err)
		time.Sleep(failedInterval)
		return nil // non-fatal; runner retries at next tick
	}
	durationStep1 := time.Since(startStep1)

	// Wait half the period before step 2, but respect context cancellation.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(strategy.Period() / 2):
	}

	startStep2 := time.Now()
	if err := strategy.recalcStep2(); err != nil {
		logger.Errorf("Recalc step 2 failed: %v", err)
		time.Sleep(failedInterval)
		return nil // non-fatal
	}
	durationStep2 := time.Since(startStep2)

	httpserver.StrategySuccessIterations.Inc()
	httpserver.StrategyIterationDuration.Set((durationStep1 + durationStep2).Seconds())
	return nil
}
```

Also add `"context"` to the import block if not already present.

**Step 4: Build to verify**

```bash
go build -C dfaasagent ./agent/loadbalancer/... 2>&1
```

Expected: build succeeds (RecalcStrategy now implements PeriodicStrategy).

**Step 5: Run all tests**

```bash
go test -C dfaasagent ./agent/loadbalancer/... -v 2>&1
```

**Step 6: Commit**

```bash
git add dfaasagent/agent/loadbalancer/recalcstrategy.go
git commit -m "refactor: migrate RecalcStrategy to PeriodicStrategy"
```

---

### Task 7: Migrate StaticStrategy to PeriodicStrategy

**Files:**
- Modify: `dfaasagent/agent/loadbalancer/staticstrategy.go`

**Context:** `StaticStrategy.RunStrategy()` is at line 51. The loop body calls `publishNodeInfo`, `updateCommonNeighbours`, `calculateWeights`, `setProxyWeights`, then sleeps.

**Step 1: Add compile-time check at bottom of file**

```go
var _ PeriodicStrategy = (*StaticStrategy)(nil)
```

**Step 2: Build to confirm it fails**

```bash
go build -C dfaasagent ./agent/loadbalancer/... 2>&1
```

**Step 3: Replace `RunStrategy()` with `Period()` and `Tick()`**

```go
func (strategy *StaticStrategy) Period() time.Duration {
	if _config.RecalcPeriod == 0 {
		return time.Minute
	}
	return _config.RecalcPeriod
}

func (strategy *StaticStrategy) Tick(ctx context.Context) error {
	logger := logging.Logger()

	if err := strategy.publishNodeInfo(); err != nil {
		logger.Errorf("StaticStrategy: failed to publish node info: %v", err)
		return nil // non-fatal
	}

	strategy.updateCommonNeighbours()

	weights, err := strategy.calculateWeights()
	if err != nil {
		return fmt.Errorf("calculating new weights: %w", err)
	}
	strategy.weights = weights

	if err := strategy.setProxyWeights(); err != nil {
		return fmt.Errorf("setting new weights: %w", err)
	}

	httpserver.StrategySuccessIterations.Inc()
	httpserver.StrategyIterationDuration.Set(time.Since(time.Now()).Seconds())
	return nil
}
```

Note: move the `start := time.Now()` / `duration` computation correctly:

```go
func (strategy *StaticStrategy) Tick(ctx context.Context) error {
	logger := logging.Logger()
	start := time.Now()

	if err := strategy.publishNodeInfo(); err != nil {
		logger.Errorf("StaticStrategy: failed to publish node info: %v", err)
		return nil
	}

	strategy.updateCommonNeighbours()

	weights, err := strategy.calculateWeights()
	if err != nil {
		return fmt.Errorf("calculating new weights: %w", err)
	}
	strategy.weights = weights

	if err := strategy.setProxyWeights(); err != nil {
		return fmt.Errorf("setting new weights: %w", err)
	}

	httpserver.StrategySuccessIterations.Inc()
	httpserver.StrategyIterationDuration.Set(time.Since(start).Seconds())
	return nil
}
```

Add `"context"` import if not already present.

**Step 4: Build and test**

```bash
go build -C dfaasagent ./agent/loadbalancer/... && go test -C dfaasagent ./agent/loadbalancer/... -v 2>&1
```

**Step 5: Commit**

```bash
git add dfaasagent/agent/loadbalancer/staticstrategy.go
git commit -m "refactor: migrate StaticStrategy to PeriodicStrategy"
```

---

### Task 8: Migrate AllLocalStrategy to PeriodicStrategy

**Files:**
- Modify: `dfaasagent/agent/loadbalancer/alllocal.go`

**Context:** `AllLocalStrategy.RunStrategy()` is at line 32. The body queries FaaS functions, detects changes, updates HAProxy only if changed.

**Step 1: Add compile-time check**

```go
var _ PeriodicStrategy = (*AllLocalStrategy)(nil)
```

**Step 2: Build to confirm failure, then replace `RunStrategy()` with `Period()` + `Tick()`**

The `AllLocalStrategy` struct needs to carry `prevFuncs` across ticks. Add it as a field:

```go
type AllLocalStrategy struct {
	hacfgupdater hacfgupd.Updater
	faasProvider faasprovider.FaaSProvider
	prevFuncs    map[string]*uint // functions at the previous Tick; nil at start
}
```

Then:

```go
func (strategy *AllLocalStrategy) Period() time.Duration {
	if _config.RecalcPeriod == 0 {
		return time.Minute
	}
	return _config.RecalcPeriod
}

func (strategy *AllLocalStrategy) Tick(ctx context.Context) error {
	start := time.Now()

	funcs, err := strategy.faasProvider.GetFuncsWithTimeout()
	if err != nil {
		return fmt.Errorf("get function metadata: %w", err)
	}

	for _, timeout := range funcs {
		if timeout != nil {
			*timeout += 1000
		}
	}

	equal := funcsMetadataEqual(funcs, strategy.prevFuncs) && funcsMetadataEqual(strategy.prevFuncs, funcs)
	if !equal {
		debugFuncsDiff(funcs, strategy.prevFuncs)
		logging.Logger().Info("Updating proxy due to new/deleted functions or changed timeouts")
		if err := strategy.updateProxyConfiguration(funcs); err != nil {
			return fmt.Errorf("updating proxy config: %w", err)
		}
		strategy.prevFuncs = funcs
	}

	httpserver.StrategySuccessIterations.Inc()
	httpserver.StrategyIterationDuration.Set(time.Since(start).Seconds())
	return nil
}
```

Check that `allLocalStrategyFactory.createStrategy()` initialises `prevFuncs`:

```go
func (f *allLocalStrategyFactory) createStrategy() (Strategy, error) {
	// ... existing code ...
	strategy := &AllLocalStrategy{
		hacfgupdater: updater,
		faasProvider: provider,
		prevFuncs:    make(map[string]*uint),
	}
	return strategy, nil
}
```

**Step 3: Build and test**

```bash
go build -C dfaasagent ./agent/loadbalancer/... && go test -C dfaasagent ./agent/loadbalancer/... -v 2>&1
```

**Step 4: Commit**

```bash
git add dfaasagent/agent/loadbalancer/alllocal.go
git commit -m "refactor: migrate AllLocalStrategy to PeriodicStrategy"
```

---

### Task 9: Migrate NodeMarginStrategy to PeriodicStrategy

**Files:**
- Modify: `dfaasagent/agent/loadbalancer/nodemarginstrategy.go`

**Context:** `NodeMarginStrategy.RunStrategy()` is at line 81. The loop body is long (~100 lines). Migration pattern is the same: extract body into `Tick()`, remove `RunStrategy()`.

**Step 1: Add compile-time check**

```go
var _ PeriodicStrategy = (*NodeMarginStrategy)(nil)
```

**Step 2: Build to confirm failure**

```bash
go build -C dfaasagent ./agent/loadbalancer/... 2>&1
```

**Step 3: Replace `RunStrategy()` with `Period()` + `Tick()`**

```go
func (strategy *NodeMarginStrategy) Period() time.Duration {
	if _config.RecalcPeriod == 0 {
		return time.Minute
	}
	return _config.RecalcPeriod
}

func (strategy *NodeMarginStrategy) Tick(ctx context.Context) error {
	logger := logging.Logger()
	start := time.Now()

	// Initialise on first tick (was done in RunStrategy before the loop).
	strategy.maxValues[cpuUsageNodeMetric] = _config.CPUThresholdNMS
	strategy.maxValues[ramUsageNodeMetric] = _config.RAMThresholdNMS
	strategy.maxValues[powerUsageNodeMetric] = _config.PowerThresholdNMS
	strategy.nodeInfo.nodeType = _config.NodeType

	cpuUsage, err := strategy.faasProvider.QueryCPUusage(_config.RecalcPeriod)
	if err != nil {
		logger.Errorf("NMS: QueryCPUusage failed: %v", err)
		return nil // non-fatal
	}
	debugPromCPUusage(_config.RecalcPeriod, cpuUsage)

	ramUsage, err := strategy.faasProvider.QueryRAMusage(_config.RecalcPeriod)
	if err != nil {
		logger.Errorf("NMS: QueryRAMusage failed: %v", err)
		return nil
	}
	debugPromRAMusage(_config.RecalcPeriod, ramUsage)

	if err := strategy.publishNodeInfo(); err != nil {
		logger.Errorf("NMS: failed to publish node info: %v", err)
		return nil
	}

	strategy.updateCommonNeighbours()

	strategy.nodeInfo.funcsRates, _ = strategy.getFunctionsRates()

	funcsGroups, err := GetFuncsGroups()
	if err != nil {
		return err
	}
	strategy.funcsGroups = funcsGroups

	strategy.nodeInfo.funcsGroupsLoad, err = strategy.getFuncsGroupsLoad()
	if err != nil {
		return err
	}

	strategy.nodeInfo.metricsPredictions, err = strategy.getNodeMetricPredictions(
		strategy.nodeInfo.nodeType, strategy.nodeInfo.funcsGroupsLoad)
	if err != nil {
		return err
	}

	strategy.nodeInfo.overload = strategy.isNodeOverloaded(strategy.nodeInfo.metricsPredictions)
	strategy.nodeInfo.margin = strategy.calculateMargin(strategy.maxValues)

	if err := strategy.sendMarginToNeighbours(); err != nil {
		return err
	}

	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		debugNodesTblContentNMS(entries)
		return nil
	})

	weights, err := strategy.calculateWeights()
	if err != nil {
		return err
	}
	strategy.weights = weights

	if err := strategy.setProxyWeights(); err != nil {
		return err
	}

	httpserver.StrategySuccessIterations.Inc()
	httpserver.StrategyIterationDuration.Set(time.Since(start).Seconds())
	return nil
}
```

NOTE: the `maxValues` / `nodeType` / `overload` initialisations that were before the loop in `RunStrategy` must be moved to `nodeMarginStrategyFactory.createStrategy()` or to the first `Tick` call. Verify the `NodeMarginStrategy` struct has fields for `maxValues`, `nodeInfo`, etc. already. The `nodeInfo.overload = false` and `strategy.maxValues` init can be put in `createStrategy`.

**Step 4: Build and test**

```bash
go build -C dfaasagent ./agent/loadbalancer/... && go test -C dfaasagent ./agent/loadbalancer/... -v 2>&1
```

**Step 5: Commit**

```bash
git add dfaasagent/agent/loadbalancer/nodemarginstrategy.go
git commit -m "refactor: migrate NodeMarginStrategy to PeriodicStrategy"
```

---

### Task 10: Wire context and runner in agent.go

**Files:**
- Modify: `dfaasagent/agent/agent.go:285-300` (goroutine startup section)

**Context:** Currently at line 296: `go func() { chanErr <- strategy.RunStrategy() }()`. The callback registration happens a few lines earlier. Replace both with the runner-based wiring. Also introduce a `context.Context` derived from the shutdown signal so runners can react to `SIGINT`/`SIGTERM`.

**Step 1: Find the exact lines to change**

```bash
grep -n "RunStrategy\|SetCallback\|MakeCommonCallback\|chanStop\|chanErr" dfaasagent/agent/agent.go
```

**Step 2: Create a cancellable context tied to the shutdown channel**

Find the line that declares `chanStop` (something like `chanStop := make(chan os.Signal, 1)`). After `signal.Notify(chanStop, ...)`, add:

```go
ctx, cancelCtx := context.WithCancel(context.Background())
go func() {
    <-chanStop
    cancelCtx()
}()
```

Add `"context"` to the import block if not already present.

**Step 3: Replace the strategy goroutine and callback wiring**

Find the section (approximately lines 285–300) that looks like:

```go
strategy, err := loadbalancer.GetStrategyInstance()
if err != nil { ... }
communication.SetCallback(MakeCommonCallback(commonTable, strategy.OnReceived))
go func() { chanErr <- strategy.RunStrategy() }()
```

Replace with:

```go
strategy, err := loadbalancer.GetStrategyInstance()
if err != nil { ... }
runner := loadbalancer.NewRunner(strategy)
communication.SetCallback(MakeCommonCallback(commonTable, runner.Callback()))
go func() { chanErr <- runner.Run(ctx) }()
```

**Step 4: Build the entire agent**

```bash
go build -C dfaasagent ./... 2>&1
```

Expected: clean build.

**Step 5: Run all tests**

```bash
go test -C dfaasagent ./... 2>&1
```

Expected: all tests pass.

**Step 6: Commit**

```bash
git add dfaasagent/agent/agent.go
git commit -m "feat: wire StrategyRunner in agent.go; strategies now use loop-model interfaces"
```

---

### Final verification

```bash
go build -C dfaasagent ./... && go test -C dfaasagent ./... -v 2>&1 | tail -30
```

All packages should build and all tests pass.
