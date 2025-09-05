// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package handles the mDNS peer discovery process
package mdns

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"

	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
)

var service mdns.Service

// notifee implements the mdns.Notifee interface. It receives notifications
// about discovered peers via mDNS.
type notifee struct {
	host host.Host
}

// HandlePeerFound is called when a new peer is found via mDNS.
func (n *notifee) HandlePeerFound(peerInfo peer.AddrInfo) {
	logger := logging.Logger()
	logger.Debug(fmt.Sprintf("Found peer %q, connecting...", peerInfo))

	if err := n.host.Connect(context.Background(), peerInfo); err != nil {
		logger.Error(fmt.Sprintf("Connecting to a discovered node (skipping): %v", err))
		return
	}

	logger.Debug(fmt.Sprintf("Connected to discovered node %q", peerInfo))
}

// Initialize builds the mDNS service and starts it.
func Initialize(p2pHost host.Host, rendezvous string) error {
	logger := logging.Logger()

	if service != nil {
		logger.Warn("mDNS discovery service already started!")
		return nil
	}

	service = mdns.NewMdnsService(p2pHost, rendezvous, &notifee{host: p2pHost})

	// Start the mDNS service with a peer found callback.
	logger.Info("Starting mDNS discovery service")
	if err := service.Start(); err != nil {
		return fmt.Errorf("starting mDNS service discovery: %w", err)
	}
	return nil
}

// Stop stops the mDNS service.
func Stop() error {
	if service == nil {
		return fmt.Errorf("stopping a mDNS service not started")
	}

	if err := service.Close(); err != nil {
		return fmt.Errorf("stopping mDNS service: %w", err)
	}
	return nil
}
