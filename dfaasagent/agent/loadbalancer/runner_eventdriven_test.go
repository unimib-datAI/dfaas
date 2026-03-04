// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/loadbalancer"
)

// fakeEvent builds a *pubsub.Message whose data contains a common-vocabulary
// header with the given msgType.
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

	assert.Equal(t, int32(1), s.reactCalled.Load())
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

	assert.Equal(t, int32(0), s.reactCalled.Load())
}

func TestEventDrivenRunner_DebounceCollapsesEvents(t *testing.T) {
	s := &debounceEventDriven{}
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runner.Run(ctx)

	cb := runner.Callback()
	// Send 3 events rapidly.
	for i := 0; i < 3; i++ {
		require.NoError(t, cb(fakeEvent("test_event")))
	}

	// Wait longer than the debounce window (40ms); 200ms gives 5x headroom for CI reliability.
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(1), s.reactCalled.Load(), "with a capacity-1 event channel, only 1 of the 3 rapid events reaches React (the others are dropped at the channel)")
}

func TestEventDrivenRunner_StopsOnContextCancel(t *testing.T) {
	s := &mockEventDriven{}
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

// debounceEventDriven is a test-only EventDrivenStrategy with a 40ms debounce.
// reactCalled is accessed atomically so the race detector is satisfied.
type debounceEventDriven struct {
	reactCalled atomic.Int32
}

func (d *debounceEventDriven) OnReceived(_ *pubsub.Message) error { return nil }
func (d *debounceEventDriven) TriggerEvents() []string            { return []string{"test_event"} }
func (d *debounceEventDriven) Debounce() time.Duration            { return 40 * time.Millisecond }
func (d *debounceEventDriven) React(_ context.Context, _ loadbalancer.StrategyEvent) error {
	d.reactCalled.Add(1)
	return nil
}

var _ loadbalancer.EventDrivenStrategy = (*debounceEventDriven)(nil)
