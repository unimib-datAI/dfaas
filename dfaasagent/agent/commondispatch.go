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

// msgTypeEnvelope extracts the msg_type field from the common vocabulary
// message JSON. The field is nested under "header" in all common messages.
type msgTypeEnvelope struct {
	Header struct {
		MsgType string `json:"msg_type"`
	} `json:"header"`
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
			if _, isCommon := communication.CommonMsgTypes[env.Header.MsgType]; isCommon {
				dispatchCommon(tbl, env.Header.MsgType, data)
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
