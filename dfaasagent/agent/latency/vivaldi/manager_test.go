// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package vivaldi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/serf/coordinate"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

func makeHostPair(t *testing.T) (host.Host, host.Host) {
	t.Helper()
	ctx := context.Background()

	h1, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = h1.Close() })

	h2, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = h2.Close() })

	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	require.NoError(t, h1.Connect(ctx, h2.Peerstore().PeerInfo(h2.ID())))

	return h1, h2
}

func addHeartbeat(tbl *nodestbl.TableCommon, peerID peer.ID) {
	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header: msgtypes.MsgHeader{
			MsgType:   msgtypes.TypeHeartbeat,
			SenderID:  peerID.String(),
			Timestamp: time.Now(),
		},
		HAProxyHost: "127.0.0.1",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	})
}

func addCoordinate(tbl *nodestbl.TableCommon, peerID peer.ID, coord *coordinate.Coordinate) {
	tbl.UpdateFromCoordinate(msgtypes.MsgCoordinate{
		Header: msgtypes.MsgHeader{
			MsgType:   msgtypes.TypeCoordinate,
			SenderID:  peerID.String(),
			Timestamp: time.Now(),
		},
		Coordinate: coord,
	})
}

func TestManagerTick_SkipsPeersWithoutCoordinate(t *testing.T) {
	h1, h2 := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	addHeartbeat(tbl, h2.ID())

	manager, err := NewManager(h1, tbl, time.Second, time.Second)
	require.NoError(t, err)

	probeCalls := 0
	manager.probeFn = func(ctx context.Context, peerID peer.ID) (time.Duration, error) {
		probeCalls++
		return 0, nil
	}
	manager.publishFn = func(msg interface{}) error { return nil }

	require.NoError(t, manager.Tick(context.Background()))
	assert.Equal(t, 0, probeCalls)
}

func TestNewManager_UsesDefaultTimingsWhenZero(t *testing.T) {
	h1, _ := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	manager, err := NewManager(h1, tbl, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, manager.probeInterval)
	assert.Equal(t, 5*time.Second, manager.probeTimeout)
}

func TestManagerTick_UpdatesLatencyFromProbe(t *testing.T) {
	h1, h2 := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	addHeartbeat(tbl, h2.ID())

	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Height = 0.25
	addCoordinate(tbl, h2.ID(), coord)

	manager, err := NewManager(h1, tbl, time.Second, time.Second)
	require.NoError(t, err)
	manager.probeFn = func(ctx context.Context, peerID peer.ID) (time.Duration, error) {
		require.Equal(t, h2.ID(), peerID)
		return 12 * time.Millisecond, nil
	}
	manager.publishFn = func(msg interface{}) error { return nil }

	require.NoError(t, manager.Tick(context.Background()))

	entry, ok := tbl.GetLiveEntry(h2.ID().String())
	require.True(t, ok)
	assert.Equal(t, 12*time.Millisecond, entry.MeasuredRTT)
	assert.Greater(t, entry.EstimatedRTT, time.Duration(0))
	assert.False(t, entry.LatencyUpdatedAt.IsZero())
}

func TestManagerOnDisconnected_ClearsLatencyAndForgetsPeer(t *testing.T) {
	h1, h2 := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	addHeartbeat(tbl, h2.ID())

	manager, err := NewManager(h1, tbl, time.Second, time.Second)
	require.NoError(t, err)
	manager.handleConnect(h2.ID())

	manager.mu.Lock()
	_, err = manager.client.Update(h2.ID().String(), coordinate.NewCoordinate(coordinate.DefaultConfig()), 15*time.Millisecond)
	manager.mu.Unlock()
	require.NoError(t, err)
	tbl.UpdateLatency(h2.ID().String(), 15*time.Millisecond, 10*time.Millisecond, time.Now())

	manager.handleDisconnect(h2.ID())

	entry, ok := tbl.GetLiveEntry(h2.ID().String())
	require.True(t, ok)
	assert.Zero(t, entry.MeasuredRTT)
	assert.Zero(t, entry.EstimatedRTT)
	assert.True(t, entry.LatencyUpdatedAt.IsZero())
	assert.NotContains(t, manager.observedPeers, h2.ID().String())
}

func TestManagerOnConnected_StartsTrackingPeer(t *testing.T) {
	h1, h2 := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	manager, err := NewManager(h1, tbl, time.Second, time.Second)
	require.NoError(t, err)

	manager.notify.Connected(h1.Network(), h1.Network().ConnsToPeer(h2.ID())[0])

	assert.Contains(t, manager.observedPeers, h2.ID().String())
}

func TestManagerTick_SkipsIncompatibleCoordinates(t *testing.T) {
	h1, h2 := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	addHeartbeat(tbl, h2.ID())

	incompatible := coordinate.NewCoordinate(coordinate.DefaultConfig())
	incompatible.Vec = make([]float64, len(incompatible.Vec)+1)
	addCoordinate(tbl, h2.ID(), incompatible)

	manager, err := NewManager(h1, tbl, time.Second, time.Second)
	require.NoError(t, err)

	probeCalls := 0
	manager.probeFn = func(ctx context.Context, peerID peer.ID) (time.Duration, error) {
		probeCalls++
		return 5 * time.Millisecond, nil
	}
	manager.publishFn = func(msg interface{}) error { return nil }

	require.NoError(t, manager.Tick(context.Background()))
	assert.Equal(t, 0, probeCalls)
}

func TestManagerTick_WithRealPingPublishesCoordinateAndStoresLatency(t *testing.T) {
	h1, h2 := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	addHeartbeat(tbl, h2.ID())

	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Height = 0.4
	addCoordinate(tbl, h2.ID(), coord)

	manager, err := NewManager(h1, tbl, time.Second, 5*time.Second)
	require.NoError(t, err)

	var published msgtypes.MsgCoordinate
	manager.publishFn = func(msg interface{}) error {
		out, ok := msg.(msgtypes.MsgCoordinate)
		require.True(t, ok)
		published = out
		return nil
	}

	require.NoError(t, manager.Tick(context.Background()))

	assert.Equal(t, msgtypes.TypeCoordinate, published.Header.MsgType)
	assert.Equal(t, h1.ID().String(), published.Header.SenderID)
	require.NotNil(t, published.Coordinate)

	entry, ok := tbl.GetLiveEntry(h2.ID().String())
	require.True(t, ok)
	assert.Greater(t, entry.MeasuredRTT, time.Duration(0))
	assert.Greater(t, entry.EstimatedRTT, time.Duration(0))
	assert.False(t, entry.LatencyUpdatedAt.IsZero())
}

func TestManagerRun_TracksExistingPeersBeforeFirstTick(t *testing.T) {
	h1, h2 := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	manager, err := NewManager(h1, tbl, time.Second, time.Second)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	manager.tickFn = func(ctx context.Context) error {
		calls++
		assert.Contains(t, manager.observedPeers, h2.ID().String())
		cancel()
		return nil
	}

	require.NoError(t, manager.Run(ctx))
	assert.Equal(t, 1, calls)
}

func TestManagerRun_LogsTickErrorWhenLoggerInitialized(t *testing.T) {
	_, _ = logging.Initialize(false, false, false)

	h1, h2 := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	manager, err := NewManager(h1, tbl, time.Second, time.Second)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	manager.tickFn = func(ctx context.Context) error {
		calls++
		assert.Contains(t, manager.observedPeers, h2.ID().String())
		cancel()
		return errors.New("tick failed")
	}

	require.NoError(t, manager.Run(ctx))
	assert.Equal(t, 1, calls)
}

func TestManagerTick_ReturnsPublishError(t *testing.T) {
	h1, _ := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	manager, err := NewManager(h1, tbl, time.Second, time.Second)
	require.NoError(t, err)

	manager.publishFn = func(msg interface{}) error { return errors.New("publish failed") }

	err = manager.Tick(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish failed")
}

func TestManagerTick_ProbeErrorDoesNotUpdateLatency(t *testing.T) {
	h1, h2 := makeHostPair(t)
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	addHeartbeat(tbl, h2.ID())

	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Height = 0.22
	addCoordinate(tbl, h2.ID(), coord)

	manager, err := NewManager(h1, tbl, time.Second, time.Second)
	require.NoError(t, err)
	manager.publishFn = func(msg interface{}) error { return nil }
	manager.probeFn = func(ctx context.Context, peerID peer.ID) (time.Duration, error) {
		return 0, errors.New("probe failed")
	}

	require.NoError(t, manager.Tick(context.Background()))

	entry, ok := tbl.GetLiveEntry(h2.ID().String())
	require.True(t, ok)
	assert.Zero(t, entry.MeasuredRTT)
	assert.Zero(t, entry.EstimatedRTT)
	assert.True(t, entry.LatencyUpdatedAt.IsZero())
}

func TestPingPeer_ReturnsContextErrorWhenCancelled(t *testing.T) {
	orig := pingResultsFn
	defer func() { pingResultsFn = orig }()

	pingResultsFn = func(ctx context.Context, h host.Host, peerID peer.ID) <-chan ping.Result {
		ch := make(chan ping.Result)
		go func() {
			<-ctx.Done()
		}()
		return ch
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rtt, err := pingPeer(ctx, nil, "")
	assert.Zero(t, rtt)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestPingPeer_ReturnsCanceledWhenChannelClosed(t *testing.T) {
	orig := pingResultsFn
	defer func() { pingResultsFn = orig }()

	pingResultsFn = func(ctx context.Context, h host.Host, peerID peer.ID) <-chan ping.Result {
		ch := make(chan ping.Result)
		close(ch)
		return ch
	}

	rtt, err := pingPeer(context.Background(), nil, "")
	assert.Zero(t, rtt)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}
