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
// ticker stops. It should be run in a goroutine.
//
// If HeartbeatInterval is zero (not configured), it defaults to 10 seconds.
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
			HAProxyPort: uint16(cfg.HAProxyPort),
			Functions:   funcs,
		}

		if err := communication.MarshAndPublish(msg); err != nil {
			logger.Warnf("heartbeat: publish failed: %v", err)
		}
	}

	return nil
}
