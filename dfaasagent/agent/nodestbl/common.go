// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package nodestbl

import (
	"sync"
	"time"

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

// GetLiveEntries returns a snapshot of all entries that have not yet expired.
// Expired entries are removed from the table as a side effect.
func (t *TableCommon) GetLiveEntries() []EntryCommon {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	var live []EntryCommon

	for id, e := range t.entries {
		if now.Sub(e.TAlive) > t.ttl {
			delete(t.entries, id)
			continue
		}
		live = append(live, *e)
	}

	return live
}
