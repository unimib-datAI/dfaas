// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// Package msgtypes defines the common message vocabulary shared across all
// DFaaS agent load-balancing strategies.
//
// Common messages are transport-agnostic: broadcast messages travel over the
// existing GossipSub topic, directed messages travel over libp2p streams
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
// presence and the set of functions it can execute locally. Transport:
// GossipSub (broadcast).
type MsgHeartbeat struct {
	Header MsgHeader `json:"header"`

	// HAProxyHost is the public IP address peers should use to forward requests
	// to this node through HAProxy.
	HAProxyHost string `json:"haproxy_host"`

    // HAProxyPort is the public port of HAProxy on this node (valid range:
    // 1–65535).
	HAProxyPort uint16 `json:"haproxy_port"`

	// Functions lists the names of the FaaS functions deployed on this node.
	Functions []string `json:"functions"`
}

// MsgOverloadAlert is broadcast when the local node exceeds a resource
// threshold. Neighbors use it to proactively absorb offloaded traffic before
// the next strategy recalculation cycle. Transport: GossipSub (broadcast,
// event-driven).
type MsgOverloadAlert struct {
	Header MsgHeader `json:"header"`

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
	Header MsgHeader `json:"header"`

	// Function is the name of the affected FaaS function.
	Function string `json:"function"`

	// Event is the lifecycle event kind: FunctionEventDeployed or
	// FunctionEventUndeployed.
	Event string `json:"event"`
}

// MsgBackpressure is sent directly to a peer that is forwarding too much
// traffic to this node. It instructs the peer to reduce or stop offloading for
// the listed functions. Transport: libp2p stream (directed, fire-and-forget).
type MsgBackpressure struct {
	Header MsgHeader `json:"header"`

    // Functions lists the function names for which forwarding should be
    // reduced.
	Functions []string `json:"functions"`

    // Reason describes why backpressure is being applied. Accepted values:
    // "overloaded", "rate_exceeded".
	Reason string `json:"reason"`
}

// MsgOffloadRequest is sent to a specific peer to ask whether it can absorb
// additional forwarded traffic for a given function. Transport: libp2p stream
// (directed, request half of request/response).
type MsgOffloadRequest struct {
	Header MsgHeader `json:"header"`

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
	Header MsgHeader `json:"header"`

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

// MsgEnvelope is a minimal struct used to peek at the msg_type field before
// full decoding. The msg_type field is nested under "header" in all common
// vocabulary messages.
type MsgEnvelope struct {
	Header struct {
		MsgType string `json:"msg_type"`
	} `json:"header"`
}

// CommonBroadcastTypes is the set of message type discriminators that belong to
// the common broadcast vocabulary. Used by the pre-filter in commondispatch.go
// to identify common messages arriving on GossipSub.
var CommonBroadcastTypes = map[string]struct{}{
	TypeHeartbeat:     {},
	TypeOverloadAlert: {},
	TypeFunctionEvent: {},
}
