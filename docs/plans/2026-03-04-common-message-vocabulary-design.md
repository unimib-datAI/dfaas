# Common Message Vocabulary Design

**Date:** 2026-03-04
**Branch:** faas-platform-abstraction
**Status:** Approved

## Context

The DFaaS agent already has a pub/sub messaging layer (libp2p GossipSub) used by individual
strategies to exchange state with peers. Each strategy currently defines its own message types
(`MsgNodeInfoRecalc`, `MsgNodeInfoNMS`, `MsgNodeMarginInfoNMS`, `MsgNodeInfoStatic`), leading to
duplicated peer-discovery logic and no shared vocabulary for cross-cutting concerns like overload
signaling or capacity negotiation.

This design introduces a **common message vocabulary** that sits alongside existing strategy-specific
messages and is available to all current and future strategies.

## Goals

- Provide reusable message types for universal patterns (presence, overload, backpressure, offload
  negotiation).
- Avoid replacing existing strategy-specific messages — coexist alongside them.
- Support three communication patterns: periodic broadcast gossip, fire-and-forget directed, and
  directed request/response.

## Transport Architecture

### Two channels

| Channel | Technology | Usage |
|---|---|---|
| Broadcast | libp2p GossipSub (existing topic) | Periodic gossip + event-driven broadcasts |
| Directed | libp2p streams (`/dfaas/msg/1.0.0`) | Fire-and-forget + request/response to a specific peer |

The existing `Communication` struct manages the GossipSub channel. A new `DirectMessenger` component
wraps `host.NewStream()` / `host.SetStreamHandler()` for the directed channel.

### Directed channel details

- Protocol ID: `/dfaas/msg/1.0.0`
- Framing: 4-byte big-endian length prefix + JSON payload (same encoding as GossipSub messages).
- Stream lifetime: open → write message → (optionally read response) → close. Streams are not
  reused.
- Timeout: configurable via `AGENT_DIRECT_MSG_TIMEOUT` (default: 5s).

## Common Message Header

All common messages embed a shared header:

```go
// MsgHeader is embedded in every common vocabulary message.
type MsgHeader struct {
    MsgType   string    `json:"msg_type"`   // message type discriminator
    SenderID  string    `json:"sender_id"`  // libp2p peer ID of the sender
    Timestamp time.Time `json:"timestamp"`  // wall-clock time at send
}
```

## Message Catalogue

### 1. MsgHeartbeat — periodic presence announcement

**Pattern:** GossipSub broadcast, sent every `AGENT_HEARTBEAT_INTERVAL` (default: 10s).

```go
// MsgHeartbeat is broadcast periodically by every agent to announce its presence and
// the set of functions it can execute locally.
type MsgHeartbeat struct {
    MsgHeader
    HAProxyHost string   `json:"haproxy_host"` // public IP reachable by peers
    HAProxyPort uint     `json:"haproxy_port"`
    Functions   []string `json:"functions"`    // function names deployed on this node
}
```

**Purpose:** peer discovery and liveness. Strategies can use this to populate the node table
without duplicating the "who am I" fields in every strategy-specific message.

---

### 2. MsgOverloadAlert — reactive overload signal

**Pattern:** GossipSub broadcast, sent **once when CPU or RAM crosses a threshold** (not periodic).

```go
// MsgOverloadAlert is broadcast when the local node exceeds a resource threshold.
// Neighbors use it to proactively prepare to absorb offloaded traffic before the next
// strategy recalculation cycle.
type MsgOverloadAlert struct {
    MsgHeader
    Functions []string `json:"functions"`  // functions that are overloaded
    CPUUsage  float64  `json:"cpu_usage"`  // current CPU usage [0.0, 1.0]
    RAMUsage  float64  `json:"ram_usage"`  // current RAM usage [0.0, 1.0]
}
```

**Purpose:** faster reaction than waiting for the next gossip cycle. A neighbor that receives this
can immediately adjust HAProxy weights without waiting for the sender's next strategy message.

---

### 3. MsgFunctionEvent — function lifecycle notification

**Pattern:** GossipSub broadcast, sent on function deploy or undeploy.

```go
// MsgFunctionEvent notifies peers that a function has been deployed or removed on this node.
// Peers update their local node table immediately rather than waiting for the next heartbeat.
type MsgFunctionEvent struct {
    MsgHeader
    Function string `json:"function"`           // function name
    Event    string `json:"event"`              // "deployed" | "undeployed"
}
```

**Purpose:** keeps the distributed view of available functions consistent without relying on heartbeat
cadence. Useful in dynamic environments where functions are deployed/removed frequently.

---

### 4. MsgBackpressure — directed load shedding signal

**Pattern:** libp2p stream (directed), fire-and-forget to a specific peer. No response expected.

```go
// MsgBackpressure is sent directly to a peer that is forwarding too much traffic to this node.
// The sender instructs the peer to reduce or stop offloading for the listed functions.
type MsgBackpressure struct {
    MsgHeader
    Functions []string `json:"functions"` // functions for which to reduce forwarding
    Reason    string   `json:"reason"`    // "overloaded" | "rate_exceeded"
}
```

**Purpose:** faster than waiting for the peer to discover the overload via its own metrics cycle.
A strategy sends this as soon as it detects excessive inbound forwarded traffic.

---

### 5. MsgOffloadRequest / MsgOffloadResponse — capacity negotiation

**Pattern:** libp2p stream (directed), request/response between two specific peers.

```go
// MsgOffloadRequest is sent to a specific peer to ask whether it can absorb additional
// forwarded traffic for a given function.
type MsgOffloadRequest struct {
    MsgHeader
    CorrelationID string  `json:"correlation_id"` // echoed in the response
    Function      string  `json:"function"`
    RateRequested float64 `json:"rate_requested"` // desired forwarding rate [req/s]
}

// MsgOffloadResponse is the reply to a MsgOffloadRequest.
// RateAccepted == 0 means the peer cannot accept any traffic for this function.
type MsgOffloadResponse struct {
    MsgHeader
    CorrelationID string  `json:"correlation_id"`
    Function      string  `json:"function"`
    RateAccepted  float64 `json:"rate_accepted"` // capacity the peer can absorb [req/s]
    Reason        string  `json:"reason,omitempty"` // populated only on rejection
}
```

**Purpose:** explicit capacity negotiation before redirecting traffic. A strategy can ask "can you
take 50 req/s of `resize`?" and receive the actual available capacity before updating HAProxy weights.

---

## Summary Table

| Message | Pattern | Transport | Trigger |
|---|---|---|---|
| `MsgHeartbeat` | broadcast | GossipSub | periodic (`AGENT_HEARTBEAT_INTERVAL`) |
| `MsgOverloadAlert` | broadcast | GossipSub | event: resource threshold crossed |
| `MsgFunctionEvent` | broadcast | GossipSub | event: function deployed / undeployed |
| `MsgBackpressure` | fire-and-forget | libp2p stream | event: inbound rate exceeded |
| `MsgOffloadRequest/Response` | request/response | libp2p stream | on-demand by strategy |

## New Components

| Component | Location | Responsibility |
|---|---|---|
| `DirectMessenger` | `agent/communication/direct.go` | libp2p stream handler and dialer |
| Common msg types | `agent/loadbalancer/commonmsgtypes.go` | `MsgHeader` + all 5 message structs |
| `CommonNodeTable` | `agent/nodestbl/common.go` | shared TTL table populated by heartbeats |

## Existing Code — No Breaking Changes

- `Communication` (GossipSub) is unchanged; `DirectMessenger` is a parallel component.
- Existing strategy message types (`MsgNodeInfoRecalc`, etc.) are unchanged.
- `OnReceived` callback routing gains common-message cases via a shared pre-filter in `agent.go`
  before dispatching to the strategy.

## New Configuration Variables

| Variable | Default | Description |
|---|---|---|
| `AGENT_HEARTBEAT_INTERVAL` | `10s` | how often to send MsgHeartbeat |
| `AGENT_DIRECT_MSG_TIMEOUT` | `5s` | timeout for libp2p stream dial + read |
