// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package nodestbl_test

import (
	"testing"
	"time"

	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

func TestTableCommon_UpdateFromHeartbeat_StoresEntry(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	hb := msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerA", Timestamp: time.Now()},
		HAProxyHost: "10.0.0.1",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	}
	tbl.UpdateFromHeartbeat(hb)

	entries := tbl.GetLiveEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "peerA", entries[0].ID)
	assert.Equal(t, "10.0.0.1", entries[0].HAProxyHost)
	assert.Equal(t, uint16(30080), entries[0].HAProxyPort)
	assert.Equal(t, []string{"resize"}, entries[0].Functions)
}

func TestTableCommon_UpdateFromOverloadAlert_UpdatesCPURAM(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	// First register the peer via heartbeat.
	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerB", Timestamp: time.Now()},
		HAProxyHost: "10.0.0.2",
		HAProxyPort: 30080,
		Functions:   []string{"figlet"},
	})

	tbl.UpdateFromOverloadAlert(msgtypes.MsgOverloadAlert{
		Header:   msgtypes.MsgHeader{SenderID: "peerB", Timestamp: time.Now()},
		CPUUsage: 0.95,
		RAMUsage: 0.80,
	})

	entries := tbl.GetLiveEntries()
	require.Len(t, entries, 1)
	assert.InDelta(t, 0.95, entries[0].CPUUsage, 0.001)
	assert.InDelta(t, 0.80, entries[0].RAMUsage, 0.001)
}

func TestTableCommon_UpdateFromFunctionEvent_Deploy(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerC", Timestamp: time.Now()},
		HAProxyHost: "10.0.0.3",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	})

	tbl.UpdateFromFunctionEvent(msgtypes.MsgFunctionEvent{
		Header:   msgtypes.MsgHeader{SenderID: "peerC", Timestamp: time.Now()},
		Function: "figlet",
		Event:    msgtypes.FunctionEventDeployed,
	})

	entries := tbl.GetLiveEntries()
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0].Functions, "figlet")
	assert.Contains(t, entries[0].Functions, "resize")
}

func TestTableCommon_UpdateFromFunctionEvent_Undeploy(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerD", Timestamp: time.Now()},
		HAProxyHost: "10.0.0.4",
		HAProxyPort: 30080,
		Functions:   []string{"resize", "figlet"},
	})

	tbl.UpdateFromFunctionEvent(msgtypes.MsgFunctionEvent{
		Header:   msgtypes.MsgHeader{SenderID: "peerD", Timestamp: time.Now()},
		Function: "figlet",
		Event:    msgtypes.FunctionEventUndeployed,
	})

	entries := tbl.GetLiveEntries()
	require.Len(t, entries, 1)
	assert.NotContains(t, entries[0].Functions, "figlet")
	assert.Contains(t, entries[0].Functions, "resize")
}

func TestTableCommon_GetLiveEntries_ExpiresStaleEntries(t *testing.T) {
	tbl := nodestbl.NewTableCommon(50 * time.Millisecond)

	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerE", Timestamp: time.Now()},
		HAProxyHost: "10.0.0.5",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	})

	time.Sleep(100 * time.Millisecond)

	entries := tbl.GetLiveEntries()
	assert.Empty(t, entries)
}

func TestTableCommon_UpdateFromCoordinate_StoresCoordinateSnapshot(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	cfg := coordinate.DefaultConfig()
	coord := coordinate.NewCoordinate(cfg)
	coord.Height = 0.42

	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerF", Timestamp: time.Now()},
		HAProxyHost: "10.0.0.6",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	})

	tbl.UpdateFromCoordinate(msgtypes.MsgCoordinate{
		Header:     msgtypes.MsgHeader{SenderID: "peerF", Timestamp: time.Now()},
		Coordinate: coord,
	})

	entry, ok := tbl.GetLiveEntry("peerF")
	require.True(t, ok)
	require.NotNil(t, entry.Coordinate)
	assert.Equal(t, coord.Height, entry.Coordinate.Height)
	assert.False(t, entry.CoordinateUpdatedAt.IsZero())

	coord.Height = 1.0
	entryAgain, ok := tbl.GetLiveEntry("peerF")
	require.True(t, ok)
	assert.Equal(t, 0.42, entryAgain.Coordinate.Height)
}

func TestTableCommon_UpdateFromCoordinate_CreatesEntryWithoutHeartbeat(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Height = 0.31
	now := time.Now()

	tbl.UpdateFromCoordinate(msgtypes.MsgCoordinate{
		Header: msgtypes.MsgHeader{
			MsgType:   msgtypes.TypeCoordinate,
			SenderID:  "peerCoordOnly",
			Timestamp: now,
		},
		Coordinate: coord,
	})

	entry, ok := tbl.GetLiveEntry("peerCoordOnly")
	require.True(t, ok)
	assert.Equal(t, "peerCoordOnly", entry.ID)
	require.NotNil(t, entry.Coordinate)
	assert.Equal(t, 0.31, entry.Coordinate.Height)
	assert.True(t, entry.TAlive.Equal(now))
	assert.True(t, entry.CoordinateUpdatedAt.Equal(now))
	assert.Empty(t, entry.Functions)
	assert.Empty(t, entry.HAProxyHost)
	assert.Zero(t, entry.HAProxyPort)
}

func TestTableCommon_UpdateFromHeartbeat_PreservesCoordinateCreatedEarlier(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Height = 0.18

	tbl.UpdateFromCoordinate(msgtypes.MsgCoordinate{
		Header: msgtypes.MsgHeader{
			MsgType:   msgtypes.TypeCoordinate,
			SenderID:  "peerSeeded",
			Timestamp: time.Now(),
		},
		Coordinate: coord,
	})

	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerSeeded", Timestamp: time.Now().Add(time.Second)},
		HAProxyHost: "10.0.0.50",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	})

	entry, ok := tbl.GetLiveEntry("peerSeeded")
	require.True(t, ok)
	require.NotNil(t, entry.Coordinate)
	assert.Equal(t, 0.18, entry.Coordinate.Height)
	assert.Equal(t, "10.0.0.50", entry.HAProxyHost)
	assert.Equal(t, uint16(30080), entry.HAProxyPort)
	assert.Equal(t, []string{"resize"}, entry.Functions)
}

func TestTableCommon_UpdateLatency_StoresMeasuredAndEstimatedRTT(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	now := time.Now()

	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerG", Timestamp: now},
		HAProxyHost: "10.0.0.7",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	})

	tbl.UpdateLatency("peerG", 12*time.Millisecond, 10*time.Millisecond, now.Add(time.Second))

	entry, ok := tbl.GetLiveEntry("peerG")
	require.True(t, ok)
	assert.Equal(t, 12*time.Millisecond, entry.MeasuredRTT)
	assert.Equal(t, 10*time.Millisecond, entry.EstimatedRTT)
	assert.Equal(t, now.Add(time.Second), entry.LatencyUpdatedAt)
}

func TestTableCommon_ForgetLatency_KeepsLivenessData(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	now := time.Now()

	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerH", Timestamp: now},
		HAProxyHost: "10.0.0.8",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	})
	tbl.UpdateLatency("peerH", 7*time.Millisecond, 5*time.Millisecond, now)

	tbl.ForgetLatency("peerH")

	entry, ok := tbl.GetLiveEntry("peerH")
	require.True(t, ok)
	assert.Equal(t, "10.0.0.8", entry.HAProxyHost)
	assert.Equal(t, uint16(30080), entry.HAProxyPort)
	assert.Equal(t, []string{"resize"}, entry.Functions)
	assert.Zero(t, entry.MeasuredRTT)
	assert.Zero(t, entry.EstimatedRTT)
	assert.True(t, entry.LatencyUpdatedAt.IsZero())
}

func TestTableCommon_GetLiveEntries_DeepCopiesCoordinate(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)
	cfg := coordinate.DefaultConfig()
	coord := coordinate.NewCoordinate(cfg)
	coord.Height = 0.21

	tbl.UpdateFromHeartbeat(msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{SenderID: "peerI", Timestamp: time.Now()},
		HAProxyHost: "10.0.0.9",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	})
	tbl.UpdateFromCoordinate(msgtypes.MsgCoordinate{
		Header:     msgtypes.MsgHeader{SenderID: "peerI", Timestamp: time.Now()},
		Coordinate: coord,
	})

	entries := tbl.GetLiveEntries()
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].Coordinate)

	entries[0].Coordinate.Height = 0.99

	entry, ok := tbl.GetLiveEntry("peerI")
	require.True(t, ok)
	require.NotNil(t, entry.Coordinate)
	assert.Equal(t, 0.21, entry.Coordinate.Height)
}
