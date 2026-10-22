// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.
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
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discoveryUtils "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
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

// Initialize initializes the Kademlia DHT peer discovery engine.
//
// If boostrapConfig.BootstrapForce is true, the function loops until all
// bootstrap nodes are connected.
//
// Returns the IpfsDHT instance that must be closed with Close() at the end.
func Initialize(ctx context.Context, p2pHost host.Host, bootstrapConfig BootstrapConfiguration, rendezvous string, idleTime time.Duration) (*dht.IpfsDHT, error) {
	logger := logging.Logger().With("component", "kademlia_discovery")

	if idleTime == 0 {
		return nil, fmt.Errorf("kademlia idle time must be a positive duration, got 0")
	}

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	//
	// Configure the peer in server mode to ensure the P2P network remains fully
	// decentralized. This setup enables bootstrapping a DFaaS agent using a
	// single known node, after which additional peers are discovered
	// automatically.
	kadDHT, err := dht.New(p2pHost, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, fmt.Errorf("error while starting the DHT for Kademlia peer discovery: %w", err)
	}

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	err = kadDHT.Bootstrap(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while bootstrapping the DHT for Kademlia peer discovery: %w", err)
	}

	// Let's connect to the bootstrap nodes. They will tell us about the other
	// nodes in the network.

	bootstrapNodes, err := BuildBoostrapNodes(bootstrapConfig)
	if err != nil {
		return nil, fmt.Errorf("building boostrap nodes: %w", err)
	}

	var wg sync.WaitGroup
	for _, peerAddr := range bootstrapNodes {
		peerInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			return nil, fmt.Errorf("error while getting information from the bootstrap node's address \"%s\": %w", peerAddr.String(), err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				logger.Debugw("Connecting to bootstrap node",
					"address", peerAddr.String(),
					"peer_id", peerInfo.ID,
				)

				err := p2pHost.Connect(ctx, *peerInfo)
				if err == nil {
					logger.Infow("Connected to bootstrap node",
						"peer_id", peerInfo.ID,
						"address", peerAddr.String(),
					)
					return
				}

				logger.Warnw("Connection failed to a bootstrap node",
					"peer_id", peerInfo.ID,
					"address", peerAddr.String(),
					"error", err,
				)

				if bootstrapConfig.BootstrapForce {
					logger.Infow("Waiting before retrying to connect to bootstrap node",
						"peer_id", peerInfo.ID,
						"address", peerAddr.String(),
						"retry_delay", 5*time.Second,
					)
					time.Sleep(5 * time.Second)
					continue
				}

				return
			}
		}()
	}
	wg.Wait()

	// Announcing ourself on the Kademlia network.
	routingDisc := discovery.NewRoutingDiscovery(kadDHT)
	discoveryUtils.Advertise(ctx, routingDisc, rendezvous)

	// If everything successful, set the package's static vars
	_ctx = ctx
	_p2pHost = p2pHost
	_rendezvous = rendezvous
	_idleTime = idleTime
	_routingDisc = routingDisc

	return kadDHT, nil
}

// RunDiscovery runs the discovery process. It should run in a goroutine.
func RunDiscovery() error {
	logger := logging.Logger().With("component", "kademlia_discovery")

	for {
		logger.Debugw("Current connected peers")

		for _, conn := range _p2pHost.Network().Conns() {
			logger.Debugw("Connected peer",
				"peer_id", conn.RemotePeer(),
				"address", conn.RemoteMultiaddr().String(),
			)
		}

		logger.Debugw("Searching for other peers", "rendezvous", _rendezvous)

		peerChan, err := _routingDisc.FindPeers(_ctx, _rendezvous)
		if err != nil {
			return fmt.Errorf("searching for peers via Kademlia discovery: %w", err)
		}

		for peerInfo := range peerChan {
			// Ignore ourselves.
			if peerInfo.ID == _p2pHost.ID() {
				continue
			}

			// Skip peer if already connected.
			if _p2pHost.Network().Connectedness(peerInfo.ID) == network.Connected {
				continue
			}

			logger.Debugw("Found a new peer",
				"peer_id", peerInfo.ID,
				"addresses", peerInfo.Addrs,
			)

			if err := _p2pHost.Connect(_ctx, peerInfo); err != nil {
				logger.Warnw("Connection to peer failed; skipping",
					"peer_id", peerInfo.ID,
					"addresses", peerInfo.Addrs,
					"error", err,
				)
				continue
			}

			logger.Debugw("Connected to a new peer",
				"peer_id", peerInfo.ID,
				"addresses", peerInfo.Addrs,
			)
		}

		// Now wait a bit and relax...
		time.Sleep(_idleTime)
	}
}

func BuildBoostrapNodes(configuration BootstrapConfiguration) ([]multiaddr.Multiaddr, error) {
	var maddrs []multiaddr.Multiaddr
	var err error

	logger := logging.Logger().With("component", "kademlia_discovery")

	if !configuration.BootstrapNodes {
		logger.Debugw("Bootstrap nodes disabled")
		return maddrs, nil
	}

	if configuration.PublicBootstrapNodes {
		logger.Debugw("Using public bootstrap nodes")

		// Use libp2p public bootstrap peers list.
		maddrs = dht.DefaultBootstrapPeers
		return maddrs, nil
	}

	if len(configuration.BootstrapNodesList) > 0 {
		logger.Debugw("Using bootstrap nodes list",
			"count", len(configuration.BootstrapNodesList),
		)

		maddrs, err = maddrhelp.StringListToMultiaddrList(configuration.BootstrapNodesList)
		if err != nil {
			return nil, fmt.Errorf("converting bootstrap peers string list to multiaddrs list: %w", err)
		}
		return maddrs, nil
	}

	if configuration.BootstrapNodesFile != "" {
		logger.Debugw("Using bootstrap nodes file",
			"file", configuration.BootstrapNodesFile,
		)

		maddrs, err = maddrhelp.ParseMAddrFile(configuration.BootstrapNodesFile)
		if err != nil {
			return nil, fmt.Errorf("parsing bootstrap peers list from file: %w", err)
		}
		return maddrs, nil
	}

	return maddrs, fmt.Errorf("at least one of bootstrap public nodes, nodes list or nodes file must be provided")
}
