// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package agent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/multiformats/go-multiaddr"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/config"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"

	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/communication"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/discovery/kademlia"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/discovery/mdns"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/httpserver"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/loadbalancer"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/utils/maddrhelp"
)

//////////////////// PRIVATE VARIABLES ////////////////////

var _p2pHost host.Host

// Convert libp2p PrivKey to ed25519.PrivateKey.
func toEd25519PrivateKey(priv crypto.PrivKey) (ed25519.PrivateKey, error) {
	// Only works for Ed25519 keys.
	bytes, err := priv.Raw()
	if err != nil {
		return nil, err
	}
	// Ed25519 private keys are 64 bytes: [32 seed | 32 public]
	return ed25519.PrivateKey(bytes), nil
}

// getPrivateKey loads the libp2p Host's private key from a PEM file if it exists
// and is not empty.
//
// The file must contain the PEM-encoded Ed25519 private key in PKCS#8 format
// (i.e., it should start with "-----BEGIN PRIVATE KEY-----").
//
// If the file does not exist or is empty, the function returns (nil, nil) and
// does NOT generate or print a new key.
//
// Returns an error if the file cannot be read, is not a valid PEM file, is not
// PKCS#8 format, or does not contain an Ed25519 private key.
func getPrivateKey(filePath string) (crypto.PrivKey, error) {
	if filePath == "" {
		return nil, nil
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil || fileInfo.Size() == 0 {
		// File does not exist or is empty
		return nil, nil
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing PKCS#8 private key: %w", err)
	}

	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an Ed25519 private key")
	}

	// UnmarshalEd25519PrivateKey expects the raw ed25519.PrivateKey bytes
	prvKey, err := crypto.UnmarshalEd25519PrivateKey(edKey)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling Ed25519 private key: %w", err)
	}

	return prvKey, nil
}

// runAgent is the main function to be called once we got some very basic setup,
// such as parsed CLI flags and a usable logger
func runAgent(config config.Configuration) error {
	// Obtain the global logger object
	logger := logging.Logger()

	// Create a new context for libp2p and all the other stuff related to
	// dfaasagent
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	////////// LIBP2P INITIALIZATION //////////

	prvKeyExist := false
	prvKey, err := getPrivateKey(config.PrivateKeyFile)
	if err != nil {
		return fmt.Errorf("getting private key file: %s", err)
	}
	if prvKey != nil {
		prvKeyExist = true
		logger.Info(fmt.Sprintf("Using private key from %q", config.PrivateKeyFile))
	}

	// Construct a new libp2p Host.
	var _addresses []multiaddr.Multiaddr
	_addresses, err = maddrhelp.StringListToMultiaddrList(config.Listen)
	if err != nil {
		return fmt.Errorf("converting string list to multiaddr list: %w", err)
	}

	if prvKeyExist {
		logger.Info("Creating libp2p host with a given private key...")
		_p2pHost, err = libp2p.New(libp2p.ListenAddrs(_addresses...), libp2p.Identity(prvKey))
	} else {
		logger.Info("Creating libp2p host with a generated private key...")
		_p2pHost, err = libp2p.New(libp2p.ListenAddrs(_addresses...))
	}
	if err != nil {
		return fmt.Errorf("creating the libp2p Host: %w", err)
	}

	// Print agent's private key in PEM format.
	if !prvKeyExist {
		prvKey := _p2pHost.Peerstore().PrivKey(_p2pHost.ID())
		if prvKey != nil {
			edPriv, err := toEd25519PrivateKey(prvKey)
			if err != nil {
				return fmt.Errorf("converting libp2p private key to Go's private key: %w", err)
			}

			// Marshal to PKCS#8 DER format.
			privBytes, err := x509.MarshalPKCS8PrivateKey(edPriv)
			if err != nil {
				return fmt.Errorf("marshal private key to PKCS#8 DER format: %w", err)
			}

			// Create PEM block and print to logger.
			pemBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}

			// Encode to a buffer.
			var buf bytes.Buffer
			if err := pem.Encode(&buf, pemBlock); err != nil {
				return fmt.Errorf("encoding private key to buffer: %w", err)
			}

			// Get PEM as string and print to logger.
			pemString := buf.String()
			logger.Info(fmt.Sprintf("Libp2p generated private key (PEM):\n%s", pemString))
		} else {
			logger.Warn("Libp2p host has no private key in Peerstore")
		}
	}

	// Print this host's multiaddresses
	myMAddrs, err := maddrhelp.BuildHostFullMAddrs(_p2pHost)
	if err != nil {
		return fmt.Errorf("error while building the p2p host's multiaddress: %w", err)
	}
	logger.Info("Libp2p host started. You can connect to this host by using the following multiaddresses:")
	for i, addr := range myMAddrs {
		logger.Info("  ", i+1, ". ", addr)
	}

	////////// LOAD BALANCER INITIALIZATION //////////

	loadbalancer.Initialize(_p2pHost, config)

	// Get the Strategy instance (which is a singleton) of type
	// dependent on the strategy specified in the configuration
	var strategy loadbalancer.Strategy
	strategy, err = loadbalancer.GetStrategyInstance()
	if err != nil {
		return fmt.Errorf("error while getting strategy instance: %w", err)
	}

	////////// PUBSUB INITIALIZATION //////////

	// The PubSub initialization must be done before the Kademlia one. Otherwise
	// the agent won't be able to publish or subscribe.
	err = communication.Initialize(ctx, _p2pHost, config.PubSubTopic, strategy.OnReceived)
	if err != nil {
		return err
	}
	logger.Debug("PubSub initialization completed")

	////////// KADEMLIA DHT INITIALIZATION //////////

	bootstrapConfig := kademlia.BootstrapConfiguration{
		BootstrapNodes:       config.BootstrapNodes,
		PublicBootstrapNodes: config.PublicBootstrapNodes,
		BootstrapNodesList:   config.BootstrapNodesList,
		BootstrapNodesFile:   config.BootstrapNodesFile,
		BootstrapForce:       config.BootstrapForce,
	}

	// Kademlia and DHT initialization, with connection to bootstrap nodes
	if err := kademlia.Initialize(ctx, _p2pHost, bootstrapConfig, config.Rendezvous, config.KadIdleTime); err != nil {
		return fmt.Errorf("initializing Kademlia: %w", err)
	}
	logger.Debug("Connection to Kademlia bootstrap nodes completed")

	// mDNS initialization.
	if config.MDNSEnabled {
		if err := mdns.Initialize(_p2pHost, config.Rendezvous); err != nil {
			return err
		}

		logger.Debug("mDNS discovery service is enabled and initialized")
		logger.Warn("mDNS discovery enabled but not currently supported in Kubernetes!")
	} else {
		logger.Debug("mDNS discovery disabled")
	}

	////////// NODESTBL INITIALIZATION //////////

	nodestbl.Initialize(config)

	////////// HTTPSERVER INITIALIZATION //////////

	httpserver.Initialize(config)

	////////// GOROUTINES //////////

	chanStop := make(chan os.Signal, 1)
	signal.Notify(chanStop, syscall.SIGINT, syscall.SIGTERM)

	chanErr := make(chan error, 1)

	go func() { chanErr <- kademlia.RunDiscovery() }()

	go func() { chanErr <- communication.RunReceiver() }()

	go func() { chanErr <- strategy.RunStrategy() }()

	go func() { chanErr <- httpserver.RunHttpServer() }()

	select {
	case sig := <-chanStop:
		logger.Warn("Caught " + sig.String() + " signal. Stopping.")
		if config.MDNSEnabled {
			if err := mdns.Stop(); err != nil {
				return err
			}
		}
		_p2pHost.Close()
		return nil
	case err = <-chanErr:
		if config.MDNSEnabled {
			if err := mdns.Stop(); err != nil {
				return err
			}
		}
		_p2pHost.Close()
		return err
	}
}

//////////////////// MAIN FUNCTION ////////////////////

func Main() {
	// Initializes Go random number generator.
	rand.Seed(int64(time.Now().Nanosecond()))

	// Load configuration.
	_config, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Setup logging engine.
	logger, err := logging.Initialize(_config.DateTime, _config.DebugMode, _config.LogColors)
	if err != nil {
		log.Fatal(err)
	}

	// Run agent.
	logger.Debugf("Running agent with configuration: %+v", _config)
	if err := runAgent(_config); err != nil {
		logger.Fatal(err)
	}
}
