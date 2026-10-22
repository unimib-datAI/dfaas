// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

//////////////////// MESSAGES' STRUCT TYPES ////////////////////

// MsgText defines the format of the PubSub messages containing a bare text message
type MsgText struct {
	MsgType string

	Text string
}

// StrMsgTextType value for MsgText.MsgType
const StrMsgTextType = "text"

// MsgRecalc defines the format of the PubSub messages used by the
// Recalc strategy.
type MsgRecalc struct {
	// Will be set to MsgTypeRecalc before sending the message.
	MsgType string

	// FuncLimits is a nested structure consisting of two maps. The first is a
	// mapping from node IDs to function names, the second is a mapping from
	// function names to rate limit.
	//
	// For a received message from another node, the meaning of
	// FuncLimits[nodeID][funcName] is: if you are nodeID, you can send me a
	// limited number of requests per second for the function called funcName.
	FuncLimits map[string]map[string]float32
}

// MsgTypeRecalc value for MsgRecalc.MsgType
const MsgTypeRecalc = "recalc"

// MsgNodeInfoNMS defines the format of the PubSub messages regarding a node's
// information (for Node Margin Strategy)
type MsgNodeInfoNMS struct {
	MsgType string

	// Information for other DFaaS nodes to forward requests to this node.
	// Consists of the node's public IP address and HAProxy's public port.
	HAProxyHost string
	HAProxyPort uint

	NodeType  int
	MaxValues map[string]float64
	Functions []string
}

// StrMsgNodeInfoTypeNMS value for MsgNodeInfoNMS.MsgType
const StrMsgNodeInfoTypeNMS = "nodeinfoNMS"

// MsgNodeMarginInfoNMS defines the format of the PubSub messages regarding a node's
// margin and eventually the expected load (for Node Margin Strategy)
type MsgNodeMarginInfoNMS struct {
	MsgType string

	Margin float64
	Load   GroupsLoad
}

// StrMsgNodeMarginInfoTypeNMS value for MsgNodeMarginInfoNMS.MsgType
const StrMsgNodeMarginInfoTypeNMS = "nodemargininfoNMS"

// MsgNodeInfoStatic defines the format of the PubSub messages regarding a
// node's information (for Static Strategy).
type MsgNodeInfoStatic struct {
	MsgType string

	// Information for other DFaaS nodes to forward requests to this node.
	// Consists of the node's public IP address and HAProxy's public port.
	HAProxyHost string
	HAProxyPort uint

	Functions []string
}

// StrMsgNodeInfoTypeStatic value for MsgNodeInfoStatic.MsgType.
const StrMsgNodeInfoTypeStatic = "nodeinfoStatic"
