// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package vivaldi

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/serf/coordinate"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/utils/p2phostutils"
)

type probeFunc func(ctx context.Context, peerID peer.ID) (time.Duration, error)
type publishFunc func(msg interface{}) error
type tickFunc func(ctx context.Context) error

var pingResultsFn = func(ctx context.Context, h host.Host, peerID peer.ID) <-chan ping.Result {
	return ping.Ping(ctx, h, peerID)
}

// Manager maintains a local Vivaldi client, publishes its coordinate, and
// enriches the shared CommonNodeTable with measured and estimated RTT data.
type Manager struct {
	host          host.Host
	table         *nodestbl.TableCommon
	client        *coordinate.Client
	probeInterval time.Duration
	probeTimeout  time.Duration

	mu            sync.Mutex
	observedPeers map[string]struct{}
	probeFn       probeFunc
	publishFn     publishFunc
	tickFn        tickFunc
	notify        *network.NotifyBundle
}

// NewManager creates a Vivaldi manager for h and tbl.
func NewManager(h host.Host, tbl *nodestbl.TableCommon, probeInterval, probeTimeout time.Duration) (*Manager, error) {
	if probeInterval <= 0 {
		probeInterval = 10 * time.Second
	}
	if probeTimeout <= 0 {
		probeTimeout = 5 * time.Second
	}

	client, err := coordinate.NewClient(coordinate.DefaultConfig())
	if err != nil {
		return nil, err
	}

	m := &Manager{
		host:          h,
		table:         tbl,
		client:        client,
		probeInterval: probeInterval,
		probeTimeout:  probeTimeout,
		observedPeers: make(map[string]struct{}),
	}
	m.probeFn = func(ctx context.Context, peerID peer.ID) (time.Duration, error) {
		return pingPeer(ctx, h, peerID)
	}
	m.publishFn = communication.MarshAndPublish
	m.tickFn = m.Tick
	m.notify = &network.NotifyBundle{
		ConnectedF: func(_ network.Network, c network.Conn) {
			m.handleConnect(c.RemotePeer())
		},
		DisconnectedF: func(_ network.Network, c network.Conn) {
			m.handleDisconnect(c.RemotePeer())
		},
	}

	// Register the ping stream handler so peers can measure RTT to this node.
	ping.NewPingService(h)

	return m, nil
}

// Run publishes the local coordinate and probes connected peers until ctx is cancelled.
func (m *Manager) Run(ctx context.Context) error {
	m.host.Network().Notify(m.notify)
	defer m.host.Network().StopNotify(m.notify)

	for _, peerID := range p2phostutils.GetConnNodeIDsUniq(m.host) {
		m.handleConnect(peerID)
	}

	ticker := time.NewTicker(m.probeInterval)
	defer ticker.Stop()

	for {
		if err := m.tickFn(ctx); err != nil && ctx.Err() == nil {
			logWarnf("vivaldi: tick failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Tick publishes the local coordinate and probes all currently connected peers
// that have already broadcast a valid, compatible remote coordinate.
func (m *Manager) Tick(ctx context.Context) error {
	if err := m.publishLocalCoordinate(); err != nil {
		return err
	}

	for _, peerID := range p2phostutils.GetConnNodeIDsUniq(m.host) {
		entry, ok := m.table.GetLiveEntry(peerID.String())
		if !ok || entry.Coordinate == nil {
			continue
		}
		if !entry.Coordinate.IsValid() {
			logWarnf("vivaldi: skipping invalid coordinate from peer %s", peerID)
			continue
		}

		m.mu.Lock()
		localCoord := m.client.GetCoordinate()
		m.mu.Unlock()
		if localCoord == nil || !localCoord.IsCompatibleWith(entry.Coordinate) {
			logWarnf("vivaldi: skipping incompatible coordinate from peer %s", peerID)
			continue
		}

		probeCtx, cancel := context.WithTimeout(ctx, m.probeTimeout)
		rtt, err := m.probeFn(probeCtx, peerID)
		cancel()
		if err != nil {
			logWarnf("vivaldi: probe failed for peer %s: %v", peerID, err)
			continue
		}

		now := time.Now()
		m.mu.Lock()
		if _, err := m.client.Update(peerID.String(), entry.Coordinate, rtt); err != nil {
			m.mu.Unlock()
			logWarnf("vivaldi: coordinate update failed for peer %s: %v", peerID, err)
			continue
		}
		estimated := m.client.DistanceTo(entry.Coordinate)
		m.mu.Unlock()

		m.table.UpdateLatency(peerID.String(), rtt, estimated, now)
	}

	return nil
}

func (m *Manager) publishLocalCoordinate() error {
	m.mu.Lock()
	coord := m.client.GetCoordinate()
	m.mu.Unlock()

	return m.publishFn(msgtypes.MsgCoordinate{
		Header: msgtypes.MsgHeader{
			MsgType:   msgtypes.TypeCoordinate,
			SenderID:  m.host.ID().String(),
			Timestamp: time.Now(),
		},
		Coordinate: coord,
	})
}

func (m *Manager) handleConnect(peerID peer.ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observedPeers[peerID.String()] = struct{}{}
}

func (m *Manager) handleDisconnect(peerID peer.ID) {
	m.mu.Lock()
	m.client.ForgetNode(peerID.String())
	delete(m.observedPeers, peerID.String())
	m.mu.Unlock()
	m.table.ForgetLatency(peerID.String())
}

func pingPeer(ctx context.Context, h host.Host, peerID peer.ID) (time.Duration, error) {
	results := pingResultsFn(ctx, h, peerID)

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case res, ok := <-results:
		if !ok {
			return 0, context.Canceled
		}
		return res.RTT, res.Error
	}
}

func logWarnf(template string, args ...interface{}) {
	logger := logging.Logger()
	if logger == nil {
		return
	}
	logger.Warnf(template, args...)
}
