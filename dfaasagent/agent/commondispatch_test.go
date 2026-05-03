// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package agent_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/serf/coordinate"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

// fakeMsg constructs a *pubsub.Message carrying the given raw JSON payload.
func fakeMsg(data []byte) *pubsub.Message {
	return &pubsub.Message{Message: &pubsub_pb.Message{Data: data}}
}

// TestMakeCommonCallback_HeartbeatUpdatesTable verifies that when a MsgHeartbeat
// is received via the callback the CommonNodeTable is updated with the peer
// entry and the strategy callback is also invoked.
func TestMakeCommonCallback_HeartbeatUpdatesTable(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	strategyCalled := false
	var strategyCB communication.CBOnReceived = func(msg *pubsub.Message) error {
		strategyCalled = true
		return nil
	}

	cb := agent.MakeCommonCallback(tbl, strategyCB)

	hb := msgtypes.MsgHeartbeat{
		Header: msgtypes.MsgHeader{
			MsgType:   msgtypes.TypeHeartbeat,
			SenderID:  "peer-hb-1",
			Timestamp: time.Now(),
		},
		HAProxyHost: "192.168.1.10",
		HAProxyPort: 30080,
		Functions:   []string{"resize", "figlet"},
	}

	data, err := json.Marshal(hb)
	require.NoError(t, err)

	require.NoError(t, cb(fakeMsg(data)))

	// The strategy callback must have been called.
	assert.True(t, strategyCalled, "strategy callback was not called")

	// The table must now contain an entry for the sender.
	entries := tbl.GetLiveEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "peer-hb-1", entries[0].ID)
	assert.Equal(t, "192.168.1.10", entries[0].HAProxyHost)
	assert.Equal(t, uint16(30080), entries[0].HAProxyPort)
	assert.Equal(t, []string{"resize", "figlet"}, entries[0].Functions)
}

// TestMakeCommonCallback_UnknownTypePassesThrough verifies that a message whose
// type is not part of the common vocabulary is still forwarded to the strategy
// callback while the CommonNodeTable remains empty.
func TestMakeCommonCallback_UnknownTypePassesThrough(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	strategyCalled := false
	var strategyCB communication.CBOnReceived = func(msg *pubsub.Message) error {
		strategyCalled = true
		return nil
	}

	cb := agent.MakeCommonCallback(tbl, strategyCB)

	// Construct a strategy-specific message that is not in CommonMsgTypes.
	type strategyMsg struct {
		Header struct {
			MsgType string `json:"msg_type"`
		} `json:"header"`
		Payload string `json:"payload"`
	}
	var sm strategyMsg
	sm.Header.MsgType = "recalc_weights"
	sm.Payload = "some strategy data"

	data, err := json.Marshal(sm)
	require.NoError(t, err)

	require.NoError(t, cb(fakeMsg(data)))

	// The strategy callback must have been invoked.
	assert.True(t, strategyCalled, "strategy callback was not called for unknown type")

	// The table must remain empty because the message type is not common.
	entries := tbl.GetLiveEntries()
	assert.Empty(t, entries)
}

func TestMakeCommonCallback_CoordinateUpdatesTable(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	strategyCalled := false
	var strategyCB communication.CBOnReceived = func(msg *pubsub.Message) error {
		strategyCalled = true
		return nil
	}

	cb := agent.MakeCommonCallback(tbl, strategyCB)
	now := time.Now()
	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Height = 0.33

	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peer-coord-1", Timestamp: now},
		HAProxyHost: "192.168.1.11",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	})

	msg := msgtypes.MsgCoordinate{
		Header: msgtypes.MsgHeader{
			MsgType:   msgtypes.TypeCoordinate,
			SenderID:  "peer-coord-1",
			Timestamp: now.Add(time.Second),
		},
		Coordinate: coord,
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)
	require.NoError(t, cb(fakeMsg(data)))

	assert.True(t, strategyCalled, "strategy callback was not called")

	entry, ok := tbl.GetLiveEntry("peer-coord-1")
	require.True(t, ok)
	require.NotNil(t, entry.Coordinate)
	assert.Equal(t, 0.33, entry.Coordinate.Height)
	assert.True(t, entry.CoordinateUpdatedAt.Equal(now.Add(time.Second)))
}

func TestMakeCommonCallback_CoordinateCreatesEntryWithoutPriorHeartbeat(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	strategyCalled := false
	var strategyCB communication.CBOnReceived = func(msg *pubsub.Message) error {
		strategyCalled = true
		return nil
	}

	cb := agent.MakeCommonCallback(tbl, strategyCB)
	now := time.Now()
	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Height = 0.27

	msg := msgtypes.MsgCoordinate{
		Header: msgtypes.MsgHeader{
			MsgType:   msgtypes.TypeCoordinate,
			SenderID:  "peer-coord-2",
			Timestamp: now,
		},
		Coordinate: coord,
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)
	require.NoError(t, cb(fakeMsg(data)))

	assert.True(t, strategyCalled, "strategy callback was not called")

	entry, ok := tbl.GetLiveEntry("peer-coord-2")
	require.True(t, ok)
	require.NotNil(t, entry.Coordinate)
	assert.Equal(t, 0.27, entry.Coordinate.Height)
	assert.True(t, entry.CoordinateUpdatedAt.Equal(now))
}
