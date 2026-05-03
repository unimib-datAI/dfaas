// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package msgtypes_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
)

func TestMsgHeartbeat_JSONRoundTrip(t *testing.T) {
	orig := msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{MsgType: msgtypes.TypeHeartbeat, SenderID: "peer1", Timestamp: time.Now().Truncate(time.Second)},
		HAProxyHost: "10.0.0.1",
		HAProxyPort: 30080,
		Functions:   []string{"resize", "figlet"},
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got msgtypes.MsgHeartbeat
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestMsgOverloadAlert_JSONRoundTrip(t *testing.T) {
	orig := msgtypes.MsgOverloadAlert{
		Header:    msgtypes.MsgHeader{MsgType: msgtypes.TypeOverloadAlert, SenderID: "peer2", Timestamp: time.Now().Truncate(time.Second)},
		Functions: []string{"resize"},
		CPUUsage:  0.95,
		RAMUsage:  0.80,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got msgtypes.MsgOverloadAlert
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestMsgOffloadRequest_JSONRoundTrip(t *testing.T) {
	orig := msgtypes.MsgOffloadRequest{
		Header:        msgtypes.MsgHeader{MsgType: msgtypes.TypeOffloadRequest, SenderID: "peer3", Timestamp: time.Now().Truncate(time.Second)},
		CorrelationID: "abc-123",
		Function:      "resize",
		RateRequested: 50.0,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got msgtypes.MsgOffloadRequest
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestMsgFunctionEvent_JSONRoundTrip(t *testing.T) {
	orig := msgtypes.MsgFunctionEvent{
		Header:   msgtypes.MsgHeader{MsgType: msgtypes.TypeFunctionEvent, SenderID: "peer4", Timestamp: time.Now().Truncate(time.Second)},
		Function: "resize",
		Event:    msgtypes.FunctionEventDeployed,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got msgtypes.MsgFunctionEvent
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestMsgBackpressure_JSONRoundTrip(t *testing.T) {
	orig := msgtypes.MsgBackpressure{
		Header:    msgtypes.MsgHeader{MsgType: msgtypes.TypeBackpressure, SenderID: "peer5", Timestamp: time.Now().Truncate(time.Second)},
		Functions: []string{"resize", "figlet"},
		Reason:    "overloaded",
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got msgtypes.MsgBackpressure
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestMsgOffloadResponse_JSONRoundTrip(t *testing.T) {
	orig := msgtypes.MsgOffloadResponse{
		Header:        msgtypes.MsgHeader{MsgType: msgtypes.TypeOffloadResponse, SenderID: "peer6", Timestamp: time.Now().Truncate(time.Second)},
		CorrelationID: "abc-123",
		Function:      "resize",
		RateAccepted:  30.0,
		Reason:        "",
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got msgtypes.MsgOffloadResponse
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestMsgOffloadResponse_Rejection_JSONRoundTrip(t *testing.T) {
	// Verify that Reason (omitempty) is preserved when the response is a rejection.
	orig := msgtypes.MsgOffloadResponse{
		Header:        msgtypes.MsgHeader{MsgType: msgtypes.TypeOffloadResponse, SenderID: "peer6", Timestamp: time.Now().Truncate(time.Second)},
		CorrelationID: "abc-123",
		Function:      "resize",
		RateAccepted:  0,
		Reason:        "overloaded",
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got msgtypes.MsgOffloadResponse
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestMsgCoordinate_JSONRoundTrip(t *testing.T) {
	cfg := coordinate.DefaultConfig()
	coord := coordinate.NewCoordinate(cfg)
	coord.Height = 0.123

	orig := msgtypes.MsgCoordinate{
		Header: msgtypes.MsgHeader{
			MsgType:   msgtypes.TypeCoordinate,
			SenderID:  "peer7",
			Timestamp: time.Now().Truncate(time.Second),
		},
		Coordinate: coord,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got msgtypes.MsgCoordinate
	require.NoError(t, json.Unmarshal(data, &got))
	require.NotNil(t, got.Coordinate)
	assert.Equal(t, orig.Header, got.Header)
	assert.Equal(t, orig.Coordinate.Vec, got.Coordinate.Vec)
	assert.Equal(t, orig.Coordinate.Error, got.Coordinate.Error)
	assert.Equal(t, orig.Coordinate.Adjustment, got.Coordinate.Adjustment)
	assert.Equal(t, orig.Coordinate.Height, got.Coordinate.Height)
}
