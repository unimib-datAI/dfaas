# Common Message Vocabulary Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a common, strategy-agnostic message vocabulary to the DFaaS agent, backed by a new
libp2p stream channel for directed messages, alongside the existing GossipSub broadcast channel.

**Architecture:** New package `agent/msgtypes` holds the shared types (no inbound deps). A
`CommonNodeTable` in `agent/nodestbl` tracks live peers populated from broadcast common messages. A
`DirectMessenger` in `agent/communication` wraps libp2p streams for fire-and-forget and
request/response. A pre-filter in `agent/agent.go` intercepts common broadcasts before delegating
to the strategy's `OnReceived`.

**Tech Stack:** Go 1.24, github.com/libp2p/go-libp2p, testify

---

### Task 1: Create `agent/msgtypes` package with common message types

**Files:**
- Create: `dfaasagent/agent/msgtypes/common.go`
- Create: `dfaasagent/agent/msgtypes/common_test.go`

**Step 1: Write the failing test**

```go
// dfaasagent/agent/msgtypes/common_test.go
package msgtypes_test

import (
	"encoding/json"
	"testing"
	"time"

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

func TestMsgOffloadResponse_JSONRoundTrip(t *testing.T) {
	orig := msgtypes.MsgOffloadResponse{
		Header:        msgtypes.MsgHeader{MsgType: msgtypes.TypeOffloadResponse, SenderID: "peer4", Timestamp: time.Now().Truncate(time.Second)},
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
```

**Step 2: Run test to verify it fails**

```bash
go test -C dfaasagent ./agent/msgtypes/...
```

Expected: `cannot find package "github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"`

**Step 3: Write the implementation**

```go
// dfaasagent/agent/msgtypes/common.go

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// Package msgtypes defines the common message vocabulary shared across all
// DFaaS agent load-balancing strategies.
//
// Common messages are transport-agnostic: broadcast messages travel over the
// existing GossipSub topic; directed messages travel over libp2p streams
// (protocol /dfaas/msg/1.0.0).
package msgtypes

import "time"

// Message type discriminator constants.
const (
	// TypeHeartbeat identifies a MsgHeartbeat message.
	TypeHeartbeat = "heartbeat"

	// TypeOverloadAlert identifies a MsgOverloadAlert message.
	TypeOverloadAlert = "overload_alert"

	// TypeFunctionEvent identifies a MsgFunctionEvent message.
	TypeFunctionEvent = "function_event"

	// TypeBackpressure identifies a MsgBackpressure message.
	TypeBackpressure = "backpressure"

	// TypeOffloadRequest identifies a MsgOffloadRequest message.
	TypeOffloadRequest = "offload_request"

	// TypeOffloadResponse identifies a MsgOffloadResponse message.
	TypeOffloadResponse = "offload_response"
)

// Function event kinds used in MsgFunctionEvent.Event.
const (
	FunctionEventDeployed   = "deployed"
	FunctionEventUndeployed = "undeployed"
)

// MsgHeader is embedded in every common vocabulary message and carries the
// minimum metadata needed to identify the sender and the message kind.
type MsgHeader struct {
	// MsgType is the message type discriminator (one of the Type* constants).
	MsgType string `json:"msg_type"`

	// SenderID is the libp2p peer ID string of the sender.
	SenderID string `json:"sender_id"`

	// Timestamp is the wall-clock time at which the message was created.
	Timestamp time.Time `json:"timestamp"`
}

// MsgHeartbeat is broadcast periodically by every agent to announce its
// presence and the set of functions it can execute locally.
// Transport: GossipSub (broadcast).
type MsgHeartbeat struct {
	MsgHeader

	// HAProxyHost is the public IP address peers should use to forward requests
	// to this node through HAProxy.
	HAProxyHost string `json:"haproxy_host"`

	// HAProxyPort is the public port of HAProxy on this node.
	HAProxyPort uint `json:"haproxy_port"`

	// Functions lists the names of the FaaS functions deployed on this node.
	Functions []string `json:"functions"`
}

// MsgOverloadAlert is broadcast when the local node exceeds a resource
// threshold. Neighbors use it to proactively absorb offloaded traffic before
// the next strategy recalculation cycle.
// Transport: GossipSub (broadcast, event-driven).
type MsgOverloadAlert struct {
	MsgHeader

	// Functions lists the function names whose execution is overloaded.
	Functions []string `json:"functions"`

	// CPUUsage is the current node CPU utilization in [0.0, 1.0].
	CPUUsage float64 `json:"cpu_usage"`

	// RAMUsage is the current node RAM utilization in [0.0, 1.0].
	RAMUsage float64 `json:"ram_usage"`
}

// MsgFunctionEvent notifies peers that a function has been deployed or removed
// on this node. Peers update their local node table immediately rather than
// waiting for the next heartbeat.
// Transport: GossipSub (broadcast, event-driven).
type MsgFunctionEvent struct {
	MsgHeader

	// Function is the name of the affected FaaS function.
	Function string `json:"function"`

	// Event is the lifecycle event kind: FunctionEventDeployed or
	// FunctionEventUndeployed.
	Event string `json:"event"`
}

// MsgBackpressure is sent directly to a peer that is forwarding too much
// traffic to this node. It instructs the peer to reduce or stop offloading
// for the listed functions.
// Transport: libp2p stream (directed, fire-and-forget).
type MsgBackpressure struct {
	MsgHeader

	// Functions lists the function names for which forwarding should be reduced.
	Functions []string `json:"functions"`

	// Reason describes why backpressure is being applied.
	// Accepted values: "overloaded", "rate_exceeded".
	Reason string `json:"reason"`
}

// MsgOffloadRequest is sent to a specific peer to ask whether it can absorb
// additional forwarded traffic for a given function.
// Transport: libp2p stream (directed, request half of request/response).
type MsgOffloadRequest struct {
	MsgHeader

	// CorrelationID is an opaque token echoed verbatim in the corresponding
	// MsgOffloadResponse so the caller can match responses to requests.
	CorrelationID string `json:"correlation_id"`

	// Function is the name of the FaaS function for which capacity is needed.
	Function string `json:"function"`

	// RateRequested is the desired additional forwarding rate in requests/second.
	RateRequested float64 `json:"rate_requested"`
}

// MsgOffloadResponse is the reply to a MsgOffloadRequest.
// RateAccepted == 0 means the peer cannot accept any additional traffic for
// this function.
// Transport: libp2p stream (directed, response half of request/response).
type MsgOffloadResponse struct {
	MsgHeader

	// CorrelationID echoes the value from the originating MsgOffloadRequest.
	CorrelationID string `json:"correlation_id"`

	// Function echoes the function name from the originating MsgOffloadRequest.
	Function string `json:"function"`

	// RateAccepted is the capacity the peer can absorb in requests/second.
	// A value of 0 means the request is rejected.
	RateAccepted float64 `json:"rate_accepted"`

	// Reason is an optional human-readable explanation, populated only on
	// rejection (RateAccepted == 0).
	Reason string `json:"reason,omitempty"`
}
```

**Step 4: Run tests to verify they pass**

```bash
go test -C dfaasagent ./agent/msgtypes/... -v
```

Expected: all 4 tests PASS.

**Step 5: Commit**

```bash
git add dfaasagent/agent/msgtypes/
git commit -m "feat: add common message vocabulary types (msgtypes package)"
```

---

### Task 2: Add config fields for heartbeat interval and direct message timeout

**Files:**
- Modify: `dfaasagent/agent/config/config.go`

**Step 1: Open the file and locate the end of the `Configuration` struct**

The struct ends at line 98 (just before `OpenWhiskAPIKey`). Add two new fields immediately before the closing brace.

**Step 2: Add the two fields**

In `dfaasagent/agent/config/config.go`, inside the `Configuration` struct, add after the `OpenWhiskAPIKey` field (line 97):

```go
	// HeartbeatInterval controls how often the agent broadcasts a MsgHeartbeat
	// to announce its presence to peers. Defaults to 10s.
	HeartbeatInterval time.Duration `mapstructure:"AGENT_HEARTBEAT_INTERVAL"`

	// DirectMsgTimeout is the deadline for dialing a peer and completing a
	// directed message exchange over a libp2p stream. Defaults to 5s.
	DirectMsgTimeout time.Duration `mapstructure:"AGENT_DIRECT_MSG_TIMEOUT"`
```

**Step 3: Build to verify no compilation errors**

```bash
go build -C dfaasagent ./...
```

Expected: exits with code 0, no output.

**Step 4: Commit**

```bash
git add dfaasagent/agent/config/config.go
git commit -m "feat: add HeartbeatInterval and DirectMsgTimeout config fields"
```

---

### Task 3: Add `CommonNodeTable` to the `nodestbl` package

**Files:**
- Create: `dfaasagent/agent/nodestbl/common.go`
- Create: `dfaasagent/agent/nodestbl/common_test.go`

**Step 1: Write the failing test**

```go
// dfaasagent/agent/nodestbl/common_test.go
package nodestbl_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

func newTestTable(ttl time.Duration) *nodestbl.TableCommon {
	return nodestbl.NewTableCommon(ttl)
}

func TestTableCommon_UpdateFromHeartbeat_StoresEntry(t *testing.T) {
	tbl := newTestTable(10 * time.Second)

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
	assert.Equal(t, uint(30080), entries[0].HAProxyPort)
	assert.Equal(t, []string{"resize"}, entries[0].Functions)
}

func TestTableCommon_UpdateFromOverloadAlert_UpdatesCPURAM(t *testing.T) {
	tbl := newTestTable(10 * time.Second)

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
	tbl := newTestTable(10 * time.Second)

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
	tbl := newTestTable(10 * time.Second)

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
	tbl := newTestTable(50 * time.Millisecond)

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
```

**Step 2: Run test to verify it fails**

```bash
go test -C dfaasagent ./agent/nodestbl/... -v
```

Expected: FAIL — `nodestbl.TableCommon` and `nodestbl.NewTableCommon` undefined.

**Step 3: Write the implementation**

```go
// dfaasagent/agent/nodestbl/common.go

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

	// HAProxyPort is the public port of HAProxy on this peer.
	HAProxyPort uint

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
```

**Step 4: Run tests to verify they pass**

```bash
go test -C dfaasagent ./agent/nodestbl/... -v
```

Expected: all 5 tests PASS.

**Step 5: Commit**

```bash
git add dfaasagent/agent/nodestbl/common.go dfaasagent/agent/nodestbl/common_test.go
git commit -m "feat: add CommonNodeTable to nodestbl package"
```

---

### Task 4: Add `DirectMessenger` to the `communication` package

**Files:**
- Create: `dfaasagent/agent/communication/direct.go`
- Create: `dfaasagent/agent/communication/direct_test.go`

**Step 1: Write the failing test**

```go
// dfaasagent/agent/communication/direct_test.go
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

// makePair creates two connected libp2p hosts for testing.
func makePair(t *testing.T) (communication.DirectMessenger, communication.DirectMessenger) {
	t.Helper()
	ctx := context.Background()

	h1, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { h1.Close() })

	h2, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { h2.Close() })

	// Connect h1 → h2.
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
```

**Step 2: Run test to verify it fails**

```bash
go test -C dfaasagent ./agent/communication/... -v
```

Expected: FAIL — `communication.DirectMessenger`, `NewDirectMessenger` undefined.

**Step 3: Write the implementation**

```go
// dfaasagent/agent/communication/direct.go

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package communication

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
)

// DirectProtocol is the libp2p protocol ID used for directed dfaas messages.
const DirectProtocol = "/dfaas/msg/1.0.0"

// msgTypeEnvelope is used to peek at the msg_type field before full decoding.
type msgTypeEnvelope struct {
	MsgType string `json:"msg_type"`
}

// HandlerFunc is called when a directed message of a specific type arrives.
// It receives the raw JSON payload and may return a response object (which will
// be JSON-encoded and sent back) or nil (fire-and-forget, no response sent).
type HandlerFunc func(raw json.RawMessage) (response interface{}, err error)

// DirectMessenger provides directed peer-to-peer messaging over libp2p streams.
// It supports fire-and-forget (Send) and request/response (Request) patterns.
type DirectMessenger interface {
	// PeerID returns the string representation of this messenger's peer ID.
	PeerID() string

	// SetHandler registers a handler for messages of the given type.
	// Replaces any previously registered handler for that type.
	SetHandler(msgType string, h HandlerFunc)

	// Send delivers msg to the peer identified by peerID and returns without
	// waiting for a response (fire-and-forget).
	Send(ctx context.Context, peerID string, msg interface{}) error

	// Request delivers msg to peerID and waits for a response, returning the
	// raw JSON of the response message.
	Request(ctx context.Context, peerID string, msg interface{}) (json.RawMessage, error)
}

// directMessenger is the concrete implementation of DirectMessenger.
type directMessenger struct {
	host     host.Host
	timeout  time.Duration
	handlers map[string]HandlerFunc
}

// NewDirectMessenger creates a DirectMessenger backed by h and registers the
// libp2p stream handler for DirectProtocol.
func NewDirectMessenger(h host.Host, timeout time.Duration) DirectMessenger {
	dm := &directMessenger{
		host:     h,
		timeout:  timeout,
		handlers: make(map[string]HandlerFunc),
	}
	h.SetStreamHandler(DirectProtocol, dm.handleStream)
	return dm
}

// PeerID returns the local peer's string ID.
func (dm *directMessenger) PeerID() string {
	return dm.host.ID().String()
}

// SetHandler registers h for the given msgType.
func (dm *directMessenger) SetHandler(msgType string, h HandlerFunc) {
	dm.handlers[msgType] = h
}

// Send opens a stream to peerID, writes msg, and closes the stream.
func (dm *directMessenger) Send(ctx context.Context, peerID string, msg interface{}) error {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return errors.Wrap(err, "invalid peer ID")
	}

	ctx, cancel := context.WithTimeout(ctx, dm.timeout)
	defer cancel()

	s, err := dm.host.NewStream(ctx, pid, DirectProtocol)
	if err != nil {
		return errors.Wrap(err, "opening directed stream")
	}
	defer s.Close()

	return writeMsg(s, msg)
}

// Request opens a stream to peerID, writes msg, reads the response, and
// returns the raw JSON response bytes.
func (dm *directMessenger) Request(ctx context.Context, peerID string, msg interface{}) (json.RawMessage, error) {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid peer ID")
	}

	ctx, cancel := context.WithTimeout(ctx, dm.timeout)
	defer cancel()

	s, err := dm.host.NewStream(ctx, pid, DirectProtocol)
	if err != nil {
		return nil, errors.Wrap(err, "opening directed stream")
	}
	defer s.Close()

	if err := writeMsg(s, msg); err != nil {
		return nil, err
	}

	// Signal that we are done writing so the remote can start processing.
	if err := s.CloseWrite(); err != nil {
		return nil, errors.Wrap(err, "closing write side of stream")
	}

	return readMsg(s)
}

// handleStream is the libp2p stream handler for incoming directed messages.
func (dm *directMessenger) handleStream(s network.Stream) {
	defer s.Close()

	raw, err := readMsg(s)
	if err != nil {
		return
	}

	// Peek at the message type.
	var env msgTypeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return
	}

	h, ok := dm.handlers[env.MsgType]
	if !ok {
		// No handler registered; ignore the message.
		return
	}

	resp, err := h(raw)
	if err != nil || resp == nil {
		return
	}

	// Write response back on the same stream.
	_ = writeMsg(s, resp)
}

// writeMsg serialises msg to JSON and writes it with a 4-byte big-endian
// length prefix.
func writeMsg(w io.Writer, msg interface{}) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "marshalling directed message")
	}

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))

	if _, err := w.Write(lenBuf[:]); err != nil {
		return errors.Wrap(err, "writing message length prefix")
	}
	if _, err := w.Write(payload); err != nil {
		return errors.Wrap(err, "writing message payload")
	}
	return nil
}

// readMsg reads a length-prefixed JSON message from r and returns the raw bytes.
func readMsg(r io.Reader) (json.RawMessage, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, errors.Wrap(err, "reading message length prefix")
	}

	length := binary.BigEndian.Uint32(lenBuf[:])
	if length == 0 {
		return nil, errors.New("received zero-length message")
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, errors.Wrap(err, "reading message payload")
	}

	return json.RawMessage(payload), nil
}

// Ensure directMessenger satisfies the interface at compile time.
var _ DirectMessenger = (*directMessenger)(nil)

// CommonMsgTypes lists the broadcast message types that belong to the common
// vocabulary. Used by the pre-filter in agent.go to identify common messages.
var CommonMsgTypes = map[string]struct{}{
	msgtypes.TypeHeartbeat:    {},
	msgtypes.TypeOverloadAlert: {},
	msgtypes.TypeFunctionEvent: {},
}
```

**Step 4: Run tests to verify they pass**

```bash
go test -C dfaasagent ./agent/communication/... -v
```

Expected: all 2 tests PASS.

**Step 5: Commit**

```bash
git add dfaasagent/agent/communication/direct.go dfaasagent/agent/communication/direct_test.go
git commit -m "feat: add DirectMessenger for directed libp2p stream messaging"
```

---

### Task 5: Add common message pre-filter in the agent package

**Files:**
- Create: `dfaasagent/agent/commondispatch.go`
- Create: `dfaasagent/agent/commondispatch_test.go`

**Step 1: Write the failing test**

```go
// dfaasagent/agent/commondispatch_test.go
package agent_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

func encodedMsg(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func makePubSubMsg(data []byte) *pubsub.Message {
	return &pubsub.Message{Message: &pubsub_pb.Message{Data: data}}
}

func TestMakeCommonCallback_HeartbeatUpdatesTable(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	strategyCallCount := 0
	strategyCB := func(msg *pubsub.Message) error {
		strategyCallCount++
		return nil
	}

	cb := agent.MakeCommonCallback(tbl, strategyCB)

	hb := msgtypes.MsgHeartbeat{
		Header:      msgtypes.MsgHeader{MsgType: msgtypes.TypeHeartbeat, SenderID: "peerX", Timestamp: time.Now()},
		HAProxyHost: "10.0.1.1",
		HAProxyPort: 30080,
		Functions:   []string{"resize"},
	}

	require.NoError(t, cb(makePubSubMsg(encodedMsg(t, hb))))

	entries := tbl.GetLiveEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "peerX", entries[0].ID)
	// Strategy callback is still called.
	assert.Equal(t, 1, strategyCallCount)
}

func TestMakeCommonCallback_UnknownTypePassesThrough(t *testing.T) {
	tbl := nodestbl.NewTableCommon(10 * time.Second)

	strategyCallCount := 0
	cb := agent.MakeCommonCallback(tbl, func(msg *pubsub.Message) error {
		strategyCallCount++
		return nil
	})

	unknown := map[string]string{"msg_type": "nodeinfoRecalc", "data": "whatever"}
	require.NoError(t, cb(makePubSubMsg(encodedMsg(t, unknown))))

	assert.Equal(t, 1, strategyCallCount)
	assert.Empty(t, tbl.GetLiveEntries()) // table unchanged
}
```

Note: `pubsub_pb` refers to `github.com/libp2p/go-libp2p-pubsub/pb`. Import it as:
```go
pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
```

**Step 2: Run test to verify it fails**

```bash
go test -C dfaasagent ./agent/ -run TestMakeCommonCallback -v
```

Expected: FAIL — `agent.MakeCommonCallback` undefined.

**Step 3: Write the implementation**

```go
// dfaasagent/agent/commondispatch.go

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package agent

import (
	"encoding/json"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

// msgTypeEnvelope is used to extract the msg_type field before full decoding.
type msgTypeEnvelope struct {
	MsgType string `json:"msg_type"`
}

// MakeCommonCallback wraps a strategy's OnReceived callback with a pre-filter
// that intercepts common broadcast messages and updates the CommonNodeTable.
// The strategy callback is always invoked after the pre-filter, so strategies
// may also react to common messages if they wish.
func MakeCommonCallback(tbl *nodestbl.TableCommon, strategyCB communication.CBOnReceived) communication.CBOnReceived {
	return func(msg *pubsub.Message) error {
		data := msg.GetData()

		// Peek at the message type without full decoding.
		var env msgTypeEnvelope
		if err := json.Unmarshal(data, &env); err == nil {
			if _, isCommon := communication.CommonMsgTypes[env.MsgType]; isCommon {
				dispatchCommon(tbl, env.MsgType, data)
			}
		}

		// Always delegate to the strategy callback.
		return strategyCB(msg)
	}
}

// dispatchCommon routes a raw common broadcast message to the appropriate
// CommonNodeTable update method.
func dispatchCommon(tbl *nodestbl.TableCommon, msgType string, data []byte) {
	switch msgType {
	case msgtypes.TypeHeartbeat:
		var msg msgtypes.MsgHeartbeat
		if err := json.Unmarshal(data, &msg); err == nil {
			tbl.UpdateFromHeartbeat(msg)
		}

	case msgtypes.TypeOverloadAlert:
		var msg msgtypes.MsgOverloadAlert
		if err := json.Unmarshal(data, &msg); err == nil {
			tbl.UpdateFromOverloadAlert(msg)
		}

	case msgtypes.TypeFunctionEvent:
		var msg msgtypes.MsgFunctionEvent
		if err := json.Unmarshal(data, &msg); err == nil {
			tbl.UpdateFromFunctionEvent(msg)
		}
	}
}
```

**Step 4: Run tests to verify they pass**

```bash
go test -C dfaasagent ./agent/ -run TestMakeCommonCallback -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add dfaasagent/agent/commondispatch.go dfaasagent/agent/commondispatch_test.go
git commit -m "feat: add common message pre-filter (MakeCommonCallback)"
```

---

### Task 6: Wire DirectMessenger and CommonNodeTable into agent.go

**Files:**
- Modify: `dfaasagent/agent/agent.go`

**Step 1: Locate the initialization section in `runAgent`**

The relevant section is around lines 203–210 where `communication.Initialize` is called:

```go
err = communication.Initialize(ctx, _p2pHost, config.PubSubTopic, strategy.OnReceived)
```

**Step 2: Apply the changes**

Replace the PubSub initialization block (lines 203–210) with the following:

```go
	////////// COMMON NODE TABLE //////////

	// TTL for common peer entries: three missed heartbeats before expiry.
	heartbeatTTL := 3 * config.HeartbeatInterval
	if heartbeatTTL == 0 {
		heartbeatTTL = 30 * time.Second // safe default when interval is not configured
	}
	commonTable := nodestbl.NewTableCommon(heartbeatTTL)

	////////// DIRECT MESSENGER //////////

	directTimeout := config.DirectMsgTimeout
	if directTimeout == 0 {
		directTimeout = 5 * time.Second
	}
	_directMessenger = communication.NewDirectMessenger(_p2pHost, directTimeout)

	////////// PUBSUB INITIALIZATION //////////

	// Wrap the strategy callback with the common message pre-filter so that
	// heartbeats, overload alerts, and function events update the shared
	// CommonNodeTable before being forwarded to the strategy.
	commonCB := MakeCommonCallback(commonTable, strategy.OnReceived)

	// The PubSub initialization must be done before the Kademlia one. Otherwise
	// the agent won't be able to publish or subscribe.
	err = communication.Initialize(ctx, _p2pHost, config.PubSubTopic, commonCB)
	if err != nil {
		return err
	}
	logger.Debug("PubSub initialization completed")
```

Also add the package-level variable near the existing `_p2pHost` declaration (line 43):

```go
var _directMessenger communication.DirectMessenger
```

**Step 3: Add `time` import if not already present**

`time` is already imported in agent.go — no change needed.

**Step 4: Build to verify compilation**

```bash
go build -C dfaasagent ./...
```

Expected: exits with code 0.

**Step 5: Run all tests**

```bash
go test -C dfaasagent ./... -v
```

Expected: all tests PASS.

**Step 6: Commit**

```bash
git add dfaasagent/agent/agent.go
git commit -m "feat: wire DirectMessenger and CommonNodeTable into agent startup"
```

---

### Task 7: Add heartbeat goroutine

**Files:**
- Create: `dfaasagent/agent/heartbeat.go`

**Step 1: Write the implementation**

There is no meaningful unit test for the heartbeat loop itself (it requires a running GossipSub),
so we build and smoke-test instead.

```go
// dfaasagent/agent/heartbeat.go

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package agent

import (
	"time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/config"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
)

// RunHeartbeat broadcasts a MsgHeartbeat at the configured interval until the
// context is cancelled. It should be run in a goroutine.
func RunHeartbeat(cfg config.Configuration, provider faasprovider.FaaSProvider) error {
	logger := logging.Logger()

	interval := cfg.HeartbeatInterval
	if interval == 0 {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		funcs, err := provider.GetFuncsNames()
		if err != nil {
			logger.Warnf("heartbeat: could not retrieve function list: %v", err)
			funcs = []string{}
		}

		msg := msgtypes.MsgHeartbeat{
			Header: msgtypes.MsgHeader{
				MsgType:   msgtypes.TypeHeartbeat,
				SenderID:  _p2pHost.ID().String(),
				Timestamp: time.Now(),
			},
			HAProxyHost: cfg.HAProxyHost,
			HAProxyPort: cfg.HAProxyPort,
			Functions:   funcs,
		}

		if err := communication.MarshAndPublish(msg); err != nil {
			logger.Warnf("heartbeat: publish failed: %v", err)
		}
	}

	return nil
}
```

**Step 2: Launch the heartbeat goroutine in `agent.go`**

In `runAgent`, inside the `////////// GOROUTINES //////////` section, add after the existing `go func()` calls (around line 271):

```go
	go func() { chanErr <- RunHeartbeat(config, faasProvider) }()
```

**Step 3: Build and run tests**

```bash
go build -C dfaasagent ./... && go test -C dfaasagent ./...
```

Expected: build succeeds, all tests pass.

**Step 4: Commit**

```bash
git add dfaasagent/agent/heartbeat.go dfaasagent/agent/agent.go
git commit -m "feat: add heartbeat goroutine that broadcasts MsgHeartbeat periodically"
```

---

### Task 8: Expose DirectMessenger to strategies via loadbalancer package

**Files:**
- Modify: `dfaasagent/agent/loadbalancer/loadbalancer.go`
- Modify: `dfaasagent/agent/agent.go`

Strategies need to call `Send` and `Request`. The cleanest approach is to store the
`DirectMessenger` in the `loadbalancer` package alongside the existing `_p2pHost`.

**Step 1: Add `_directMessenger` to loadbalancer package vars**

In `dfaasagent/agent/loadbalancer/loadbalancer.go`, add after `_p2pHost host.Host` (line 31):

```go
	// _directMessenger is used by strategies to send directed messages to peers.
	_directMessenger communication.DirectMessenger
```

Add the import:
```go
"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
```

**Step 2: Extend `Initialize` to accept the messenger**

Change the signature of `Initialize`:

```go
func Initialize(p2pHost host.Host, dm communication.DirectMessenger, config config.Configuration) {
	_p2pHost = p2pHost
	_directMessenger = dm
	_config = config
	_lock = &sync.Mutex{}
	// ... rest unchanged
}
```

**Step 3: Expose a getter for use by strategies**

Add at the end of `loadbalancer.go`:

```go
// DirectMessenger returns the DirectMessenger available to strategies for
// sending directed messages to specific peers.
func DirectMessenger() communication.DirectMessenger {
	return _directMessenger
}
```

**Step 4: Update the call site in `agent.go`**

In `runAgent`, change:

```go
loadbalancer.Initialize(_p2pHost, config)
```

to:

```go
loadbalancer.Initialize(_p2pHost, _directMessenger, config)
```

Note: `_directMessenger` must be initialised before `loadbalancer.Initialize` is called.
Move the `DirectMessenger` init block (from Task 6) to before the `loadbalancer.Initialize` call.

**Step 5: Build and test**

```bash
go build -C dfaasagent ./... && go test -C dfaasagent ./...
```

Expected: build succeeds, all tests pass.

**Step 6: Commit**

```bash
git add dfaasagent/agent/loadbalancer/loadbalancer.go dfaasagent/agent/agent.go
git commit -m "feat: expose DirectMessenger to strategies via loadbalancer package"
```
