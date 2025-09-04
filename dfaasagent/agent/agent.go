// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package agent

import (
	"context"
	cryptorand "crypto/rand"
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
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"
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

//////////////////// PRIVATE FUNCTIONS ////////////////////

// getPrivateKey loads the libp2p Host's private key from file if it exists and
// it is not empty, or generates it and writes it to the file otherwise
func getPrivateKey(filePath string) (crypto.PrivKey, error) {
	var (
		prvKey crypto.PrivKey = nil
		err    error
		data   []byte
	)

	logger := logging.Logger()

	fileInfo, fileErr := os.Stat(filePath)
	fileExists := fileErr == nil
	keyAlreadyPresent := fileExists && fileInfo.Size() > 0

	if filePath != "" && keyAlreadyPresent {
		logger.Debug("Loading the RSA private key file content from: \"", filePath, "\"")

		data, err = ioutil.ReadFile(filePath)
		if err != nil {
			return nil, errors.Wrap(err, "Error while loading the RSA private key file content")
		}

		prvKey, err = crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, errors.Wrap(err, "Error while unmarshalling the RSA private key from file")
		}
	} else {
		logger.Debug("Generating a new RSA key pair")

		prvKey, _, err = crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, cryptorand.Reader)
		if err != nil {
			return nil, errors.Wrap(err, "Error while generating a new RSA key pair")
		}

		if filePath != "" {
			logger.Debug("Writing the newly generated RSA private key to file")

			data, err = crypto.MarshalPrivateKey(prvKey)
			if err != nil {
				return nil, errors.Wrap(err, "Error while marshalling the newly generated RSA private key")
			}

			err = ioutil.WriteFile(filePath, data, 0644)
			if err != nil {
				return nil, errors.Wrap(err, "Error while writing the RSA private key to file")
			}
		}
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

	// RSA key pair for this p2p host
	prvKey, err := getPrivateKey(config.PrivateKeyFile)
	if err != nil {
		return err
	}

	// Construct a new libp2p Host
	var _addresses []multiaddr.Multiaddr
	_addresses, err = maddrhelp.StringListToMultiaddrList(config.Listen)
	if err != nil {
		return errors.Wrap(err, "Error while converting string list to multiaddr list")
	}
	_p2pHost, err = libp2p.New(ctx, libp2p.ListenAddrs(_addresses...), libp2p.Identity(prvKey))
	if err != nil {
		return errors.Wrap(err, "Error while creating the libp2p Host")
	}

	// Print this host's multiaddresses
	myMAddrs, err := maddrhelp.BuildHostFullMAddrs(_p2pHost)
	if err != nil {
		return errors.Wrap(err, "Error while building the p2p host's multiaddress")
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
		return errors.Wrap(err, "Error while getting strategy instance")
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
	err = kademlia.Initialize(
		ctx,
		_p2pHost,
		bootstrapConfig,
		config.Rendezvous,
		config.KadIdleTime,
	)

	if err != nil {
		return err
	}

	logger.Debug("Connection to Kademlia bootstrap nodes completed")

	// mDNS initialization.
	if config.MDNSInterval > 0 {
		if err := mdns.Initialize(ctx, _p2pHost, config.Rendezvous, config.MDNSInterval); err != nil {
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

	if config.MDNSInterval > 0 {
		go func() { chanErr <- mdns.RunDiscovery() }()
	}

	go func() { chanErr <- communication.RunReceiver() }()

	go func() { chanErr <- strategy.RunStrategy() }()

	go func() { chanErr <- httpserver.RunHttpServer() }()

	select {
	case sig := <-chanStop:
		logger.Warn("Caught " + sig.String() + " signal. Stopping.")
		_p2pHost.Close()
		return nil
	case err = <-chanErr:
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
