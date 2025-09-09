// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package handles the Kademlia peer discovery process
package kademlia

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/multiformats/go-multiaddr"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/utils/maddrhelp"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discoveryUtils "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"go.uber.org/zap"
)

type BootstrapConfiguration struct {
	BootstrapNodes       bool
	PublicBootstrapNodes bool
	BootstrapNodesList   []string
	BootstrapNodesFile   string
	BootstrapForce       bool
}

var _ctx context.Context
var _p2pHost host.Host
var _rendezvous string
var _idleTime time.Duration
var _routingDisc *discovery.RoutingDiscovery

// Initialize initializes the Kademlia DHT peer discovery engine. If
// bootstrapForce = true, then this function fails if any of the bootstrap peers
// cannot be contacted for some reason.
func Initialize(ctx context.Context, p2pHost host.Host, bootstrapConfig BootstrapConfiguration, rendezvous string, idleTime time.Duration) error {
	logger := logging.Logger()

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	kadDHT, err := dht.New(ctx, p2pHost)
	if err != nil {
		return fmt.Errorf("Error while starting the DHT for Kademlia peer discovery: %w", err)
	}

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	err = kadDHT.Bootstrap(ctx)
	if err != nil {
		return fmt.Errorf("Error while bootstrapping the DHT for Kademlia peer discovery: %w", err)
	}

	// Let's connect to the bootstrap nodes. They will tell us about the other
	// nodes in the network.

	bootstrapNodes, err := BuildBoostrapNodes(bootstrapConfig)
	if err != nil {
		return fmt.Errorf("building boostrap nodes: %w", err)
	}

	var wg sync.WaitGroup
	var chanErrConn = make(chan error, len(bootstrapNodes))
	for _, peerAddr := range bootstrapNodes {
		peerInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			return fmt.Errorf("Error while getting information from the bootstrap node's address \"%s\": %w", peerAddr.String(), err)
		}

		logger.Debugf("Connecting to bootstrap node: %s", peerAddr.String())
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := p2pHost.Connect(ctx, *peerInfo)
			if err != nil {
				errWrap := fmt.Errorf("Connection failed to a bootstrap node (skipping): %w", err)

				if bootstrapConfig.BootstrapForce {
					chanErrConn <- errWrap
				} else {
					logger.Error("Kademlia: ", errWrap)
				}

				return
			}

			logger.Infof("Connected to bootstrap node %s", peerInfo.String())
		}()
	}
	wg.Wait()
	select {
	case err := <-chanErrConn:
		return err
	default:
	}

	// Announcing ourself on the Kademlia network.
	routingDisc := discovery.NewRoutingDiscovery(kadDHT)
	discoveryUtils.Advertise(ctx, routingDisc, rendezvous)

	if idleTime == 0 {
		logger.Warn("Given Kademlia idle time must be a positive duration, using 1 minute by default")
		idleTime = 1 * time.Minute
	}

	// If everything successful, set the package's static vars
	_ctx = ctx
	_p2pHost = p2pHost
	_rendezvous = rendezvous
	_idleTime = idleTime
	_routingDisc = routingDisc

	return nil
}

// RunDiscovery runs the discovery process. It should run in a goroutine.
func RunDiscovery() error {
	logger := logging.Logger()

	for {
		logger.Debug("Current connected peers: ")
		for _, conn := range _p2pHost.Network().Conns() {
			logger.Debug(fmt.Sprintf("  - Peer: %s, Addr: %s\n", conn.RemotePeer(), conn.RemoteMultiaddr()))
		}

		logger.Debug("Searching for other peers...")
		peerChan, err := _routingDisc.FindPeers(_ctx, _rendezvous)
		if err != nil {
			return fmt.Errorf("searching for peers via Kademlia discovery: %w", err)
		}

		for peerInfo := range peerChan {
			// Ignore ourselves.
			if peerInfo.ID == _p2pHost.ID() {
				continue
			}

			logger.Debugf("Connecting to a new peer: ",
				zap.Any("addrs", peerInfo.Addrs),
				zap.String("id", peerInfo.ID.String()))
			if err := _p2pHost.Connect(_ctx, peerInfo); err != nil {
				logger.Error("Connection to peer (skipping): ", zap.Error(err))
				continue
			}
			logger.Debug("Connected to a new peer",
				zap.Any("addrs", peerInfo.Addrs),
				zap.String("id", peerInfo.ID.String()))
		}

		// Now wait a bit and relax...
		time.Sleep(_idleTime)
	}
}

func BuildBoostrapNodes(configuration BootstrapConfiguration) ([]multiaddr.Multiaddr, error) {
	var maddrs []multiaddr.Multiaddr
	var err error

	logger := logging.Logger()

	if !configuration.BootstrapNodes {
		logger.Debug("Bootstrap nodes disabled")
		return maddrs, nil
	}

	if configuration.PublicBootstrapNodes {
		logger.Debug("Using public bootstrap nodes")

		// Use libp2p public bootstrap peers list.
		maddrs = dht.DefaultBootstrapPeers
		return maddrs, nil
	}

	if len(configuration.BootstrapNodesList) > 0 {
		logger.Debug("Using bootstrap nodes list")

		maddrs, err = maddrhelp.StringListToMultiaddrList(configuration.BootstrapNodesList)
		if err != nil {
			return nil, fmt.Errorf("converting bootstrap peers string list to multiaddrs list: %w", err)
		}
		return maddrs, nil
	}

	if configuration.BootstrapNodesFile != "" {
		logger.Debug("Using bootstrap nodes file")

		maddrs, err = maddrhelp.ParseMAddrFile(configuration.BootstrapNodesFile)
		if err != nil {
			return nil, fmt.Errorf("parsing bootstrap peers list from file: %w", err)
		}
		return maddrs, nil
	}

	return maddrs, fmt.Errorf("at least one of bootstrap public nodes, nodes list or nodes file must be provided")
}
