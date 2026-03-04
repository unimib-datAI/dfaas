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
	// Period() == 0 should default to 1 minute.
	// With 50ms window, Tick should NOT be called.
	s := &mockPeriodic{period: 0}
	runner := loadbalancer.NewRunner(s)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	<-done

	assert.Equal(t, 0, s.tickCalled)
}
