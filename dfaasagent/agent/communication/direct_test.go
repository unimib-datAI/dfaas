// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package communication_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
)

// makePair creates two connected libp2p TCP hosts for testing.
func makePair(t *testing.T) (communication.DirectMessenger, communication.DirectMessenger) {
	t.Helper()
	ctx := context.Background()

	h1, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { h1.Close() })

	h2, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { h2.Close() })

	// Connect h1 to h2.
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	require.NoError(t, h1.Connect(ctx, h2.Peerstore().PeerInfo(h2.ID())))

	dm1 := communication.NewDirectMessenger(h1, 5*time.Second)
	dm2 := communication.NewDirectMessenger(h2, 5*time.Second)

	return dm1, dm2
}

func TestDirectMessenger_Send_FireAndForget(t *testing.T) {
	ctx := context.Background()
	dm1, dm2 := makePair(t)

	received := make(chan msgtypes.MsgBackpressure, 1)
	dm2.SetHandler(msgtypes.TypeBackpressure, func(raw json.RawMessage) (interface{}, error) {
		var msg msgtypes.MsgBackpressure
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}
		received <- msg
		return nil, nil // no response for fire-and-forget
	})

	bp := msgtypes.MsgBackpressure{
		Header:    msgtypes.MsgHeader{MsgType: msgtypes.TypeBackpressure, SenderID: dm1.PeerID(), Timestamp: time.Now()},
		Functions: []string{"resize"},
		Reason:    "overloaded",
	}
	require.NoError(t, dm1.Send(ctx, dm2.PeerID(), bp))

	select {
	case msg := <-received:
		assert.Equal(t, []string{"resize"}, msg.Functions)
		assert.Equal(t, "overloaded", msg.Reason)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: did not receive backpressure message")
	}
}

func TestDirectMessenger_Request_ReturnsResponse(t *testing.T) {
	ctx := context.Background()
	dm1, dm2 := makePair(t)

	dm2.SetHandler(msgtypes.TypeOffloadRequest, func(raw json.RawMessage) (interface{}, error) {
		var req msgtypes.MsgOffloadRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, err
		}
		return msgtypes.MsgOffloadResponse{
			Header:        msgtypes.MsgHeader{MsgType: msgtypes.TypeOffloadResponse, SenderID: dm2.PeerID(), Timestamp: time.Now()},
			CorrelationID: req.CorrelationID,
			Function:      req.Function,
			RateAccepted:  req.RateRequested / 2,
		}, nil
	})

	req := msgtypes.MsgOffloadRequest{
		Header:        msgtypes.MsgHeader{MsgType: msgtypes.TypeOffloadRequest, SenderID: dm1.PeerID(), Timestamp: time.Now()},
		CorrelationID: "test-corr",
		Function:      "resize",
		RateRequested: 100.0,
	}

	rawResp, err := dm1.Request(ctx, dm2.PeerID(), req)
	require.NoError(t, err)

	var resp msgtypes.MsgOffloadResponse
	require.NoError(t, json.Unmarshal(rawResp, &resp))
	assert.Equal(t, "test-corr", resp.CorrelationID)
	assert.InDelta(t, 50.0, resp.RateAccepted, 0.001)
}
