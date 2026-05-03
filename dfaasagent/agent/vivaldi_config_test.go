// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package agent

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/config"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/latency/vivaldi"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

func TestNewVivaldiManagerFromConfig_Disabled(t *testing.T) {
	called := false
	orig := newVivaldiManager
	defer func() { newVivaldiManager = orig }()
	newVivaldiManager = func(_ host.Host, _ *nodestbl.TableCommon, probeInterval, probeTimeout time.Duration) (*vivaldi.Manager, error) {
		called = true
		return &vivaldi.Manager{}, nil
	}

	manager, interval, timeout, err := newVivaldiManagerFromConfig(config.Configuration{}, nil, nodestbl.NewTableCommon(time.Second))
	requireNoError(t, err)
	assertNil(t, manager)
	assertDurationZero(t, interval)
	assertDurationZero(t, timeout)
	assertFalse(t, called)
}

func TestNewVivaldiManagerFromConfig_UsesFallbacks(t *testing.T) {
	var gotInterval, gotTimeout time.Duration
	orig := newVivaldiManager
	defer func() { newVivaldiManager = orig }()
	newVivaldiManager = func(_ host.Host, _ *nodestbl.TableCommon, probeInterval, probeTimeout time.Duration) (*vivaldi.Manager, error) {
		gotInterval = probeInterval
		gotTimeout = probeTimeout
		return &vivaldi.Manager{}, nil
	}

	manager, interval, timeout, err := newVivaldiManagerFromConfig(config.Configuration{
		VivaldiEnabled:    true,
		HeartbeatInterval: 7 * time.Second,
	}, nil, nodestbl.NewTableCommon(time.Second))
	requireNoError(t, err)
	assertNotNil(t, manager)
	assertEqualDuration(t, 7*time.Second, interval)
	assertEqualDuration(t, 5*time.Second, timeout)
	assertEqualDuration(t, 7*time.Second, gotInterval)
	assertEqualDuration(t, 5*time.Second, gotTimeout)
}

func TestNewVivaldiManagerFromConfig_UsesExplicitValues(t *testing.T) {
	var gotInterval, gotTimeout time.Duration
	orig := newVivaldiManager
	defer func() { newVivaldiManager = orig }()
	newVivaldiManager = func(_ host.Host, _ *nodestbl.TableCommon, probeInterval, probeTimeout time.Duration) (*vivaldi.Manager, error) {
		gotInterval = probeInterval
		gotTimeout = probeTimeout
		return &vivaldi.Manager{}, nil
	}

	manager, interval, timeout, err := newVivaldiManagerFromConfig(config.Configuration{
		VivaldiEnabled:       true,
		VivaldiProbeInterval: 3 * time.Second,
		DirectMsgTimeout:     9 * time.Second,
	}, nil, nodestbl.NewTableCommon(time.Second))
	requireNoError(t, err)
	assertNotNil(t, manager)
	assertEqualDuration(t, 3*time.Second, interval)
	assertEqualDuration(t, 9*time.Second, timeout)
	assertEqualDuration(t, 3*time.Second, gotInterval)
	assertEqualDuration(t, 9*time.Second, gotTimeout)
}

func TestNewVivaldiManagerFromConfig_ReturnsFactoryError(t *testing.T) {
	orig := newVivaldiManager
	defer func() { newVivaldiManager = orig }()
	newVivaldiManager = func(_ host.Host, _ *nodestbl.TableCommon, probeInterval, probeTimeout time.Duration) (*vivaldi.Manager, error) {
		return nil, errors.New("boom")
	}

	manager, interval, timeout, err := newVivaldiManagerFromConfig(config.Configuration{
		VivaldiEnabled: true,
	}, nil, nodestbl.NewTableCommon(time.Second))
	assertNil(t, manager)
	assertEqualDuration(t, 10*time.Second, interval)
	assertEqualDuration(t, 5*time.Second, timeout)
	assertErrorContains(t, err, "boom")
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertNil(t *testing.T, v interface{}) {
	t.Helper()
	if v == nil {
		return
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.IsNil() {
			return
		}
	}
	if v != nil {
		t.Fatalf("expected nil, got %#v", v)
	}
}

func assertNotNil(t *testing.T, v interface{}) {
	t.Helper()
	if v == nil {
		t.Fatal("expected non-nil value")
	}
}

func assertFalse(t *testing.T, v bool) {
	t.Helper()
	if v {
		t.Fatal("expected false")
	}
}

func assertDurationZero(t *testing.T, d time.Duration) {
	t.Helper()
	if d != 0 {
		t.Fatalf("expected zero duration, got %s", d)
	}
}

func assertEqualDuration(t *testing.T, want, got time.Duration) {
	t.Helper()
	if want != got {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func assertErrorContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("expected error containing %q, got %v", substr, err)
	}
}
