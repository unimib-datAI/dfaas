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

func TestHybridRunner_TickFires(t *testing.T) {
	s := &mockHybrid{} // period = 50ms
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	<-done

	// 200ms / 50ms = 4 expected ticks; assert at least 2 for CI headroom.
	assert.GreaterOrEqual(t, s.tickCalled.Load(), int32(2))
}

func TestHybridRunner_ReactFires(t *testing.T) {
	s := &mockHybrid{} // period = 50ms, TriggerEvents = ["test_event"]
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runner.Run(ctx)

	cb := runner.Callback()
	require.NoError(t, cb(fakeEvent("test_event")))

	time.Sleep(80 * time.Millisecond)
	assert.GreaterOrEqual(t, s.reactCalled.Load(), int32(1))
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
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not stop after context cancel")
	}
}
