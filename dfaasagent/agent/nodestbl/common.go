// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package nodestbl

import (
	"sync"
	"time"

	"github.com/hashicorp/serf/coordinate"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
)

// EntryCommon holds the common state tracked for a peer node, populated from
// the common message vocabulary (heartbeats, overload alerts, function events).
type EntryCommon struct {
	// ID is the libp2p peer ID of the node.
	ID string

	// TAlive is the timestamp of the last message received from this peer.
	TAlive time.Time

	// HAProxyHost is the public IP address to use when forwarding requests to
	// this peer through its HAProxy instance.
	HAProxyHost string

	// HAProxyPort is the public port of HAProxy on this peer (valid range: 1–65535).
	HAProxyPort uint16

	// Functions is the list of FaaS function names deployed on this peer.
	Functions []string

	// CPUUsage is the last reported CPU utilization in [0.0, 1.0].
	// Updated by MsgOverloadAlert; zero if no alert has been received.
	CPUUsage float64

	// RAMUsage is the last reported RAM utilization in [0.0, 1.0].
	// Updated by MsgOverloadAlert; zero if no alert has been received.
	RAMUsage float64

	// Coordinate is the last Vivaldi network coordinate broadcast by this peer.
	Coordinate *coordinate.Coordinate

	// CoordinateUpdatedAt is when Coordinate was last updated.
	CoordinateUpdatedAt time.Time

	// MeasuredRTT is the most recent libp2p RTT observed to this peer.
	MeasuredRTT time.Duration

	// EstimatedRTT is the RTT estimated from the local and peer coordinates.
	EstimatedRTT time.Duration

	// LatencyUpdatedAt is when the RTT fields were last updated.
	LatencyUpdatedAt time.Time
}

// TableCommon is a thread-safe, TTL-expiring table of peer nodes populated
// from common broadcast messages.
type TableCommon struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]*EntryCommon // keyed by peer ID
}

// NewTableCommon creates a new TableCommon where entries expire after ttl.
func NewTableCommon(ttl time.Duration) *TableCommon {
	return &TableCommon{
		ttl:     ttl,
		entries: make(map[string]*EntryCommon),
	}
}

// UpdateFromHeartbeat upserts the peer entry using data from a MsgHeartbeat.
func (t *TableCommon) UpdateFromHeartbeat(msg msgtypes.MsgHeartbeat) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e, ok := t.entries[msg.Header.SenderID]
	if !ok {
		e = &EntryCommon{ID: msg.Header.SenderID}
		t.entries[msg.Header.SenderID] = e
	}

	e.TAlive = msg.Header.Timestamp
	e.HAProxyHost = msg.HAProxyHost
	e.HAProxyPort = msg.HAProxyPort
	e.Functions = msg.Functions
}

// UpdateFromOverloadAlert updates CPU/RAM usage for the peer identified by the
// message sender. If the peer is not yet in the table, the message is ignored.
func (t *TableCommon) UpdateFromOverloadAlert(msg msgtypes.MsgOverloadAlert) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e, ok := t.entries[msg.Header.SenderID]
	if !ok {
		return
	}

	e.TAlive = msg.Header.Timestamp
	e.CPUUsage = msg.CPUUsage
	e.RAMUsage = msg.RAMUsage
}

// UpdateFromFunctionEvent adds or removes a function from the peer's function
// list. If the peer is not yet in the table, the message is ignored.
func (t *TableCommon) UpdateFromFunctionEvent(msg msgtypes.MsgFunctionEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e, ok := t.entries[msg.Header.SenderID]
	if !ok {
		return
	}

	e.TAlive = msg.Header.Timestamp

	switch msg.Event {
	case msgtypes.FunctionEventDeployed:
		// Add only if not already present.
		for _, f := range e.Functions {
			if f == msg.Function {
				return
			}
		}
		e.Functions = append(e.Functions, msg.Function)

	case msgtypes.FunctionEventUndeployed:
		filtered := e.Functions[:0]
		for _, f := range e.Functions {
			if f != msg.Function {
				filtered = append(filtered, f)
			}
		}
		e.Functions = filtered
	}
}

// UpdateFromCoordinate stores the peer's last broadcast Vivaldi coordinate.
// If the peer is not yet in the table, a skeletal entry is created so early
// coordinate broadcasts are not lost before the first heartbeat arrives.
func (t *TableCommon) UpdateFromCoordinate(msg msgtypes.MsgCoordinate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e, ok := t.entries[msg.Header.SenderID]
	if !ok {
		e = &EntryCommon{ID: msg.Header.SenderID}
		t.entries[msg.Header.SenderID] = e
	}

	e.TAlive = msg.Header.Timestamp
	e.Coordinate = cloneCoordinate(msg.Coordinate)
	e.CoordinateUpdatedAt = msg.Header.Timestamp
}

// UpdateLatency stores the latest measured and estimated RTT values for peerID.
// If the peer is not yet in the table, the update is ignored.
func (t *TableCommon) UpdateLatency(peerID string, measured, estimated time.Duration, ts time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e, ok := t.entries[peerID]
	if !ok {
		return
	}

	e.MeasuredRTT = measured
	e.EstimatedRTT = estimated
	e.LatencyUpdatedAt = ts
}

// ForgetLatency clears only the latency-derived state for peerID. Other
// liveness and routing metadata are preserved.
func (t *TableCommon) ForgetLatency(peerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e, ok := t.entries[peerID]
	if !ok {
		return
	}

	e.MeasuredRTT = 0
	e.EstimatedRTT = 0
	e.LatencyUpdatedAt = time.Time{}
}

// GetLiveEntry returns a snapshot of peerID if it exists and has not expired.
// Expired entries are removed from the table as a side effect.
func (t *TableCommon) GetLiveEntry(peerID string) (EntryCommon, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.purgeExpiredLocked(time.Now())

	e, ok := t.entries[peerID]
	if !ok {
		return EntryCommon{}, false
	}

	return snapshotEntry(e), true
}

// GetLiveEntries returns a snapshot of all entries that have not yet expired.
// Expired entries are removed from the table as a side effect.
func (t *TableCommon) GetLiveEntries() []EntryCommon {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.purgeExpiredLocked(time.Now())
	var live []EntryCommon

	for _, e := range t.entries {
		live = append(live, snapshotEntry(e))
	}

	return live
}

func (t *TableCommon) purgeExpiredLocked(now time.Time) {
	for id, e := range t.entries {
		if now.Sub(e.TAlive) > t.ttl {
			delete(t.entries, id)
		}
	}
}

func snapshotEntry(e *EntryCommon) EntryCommon {
	snapshot := *e
	snapshot.Functions = append([]string(nil), e.Functions...)
	snapshot.Coordinate = cloneCoordinate(e.Coordinate)
	return snapshot
}

func cloneCoordinate(coord *coordinate.Coordinate) *coordinate.Coordinate {
	if coord == nil {
		return nil
	}
	return coord.Clone()
}
