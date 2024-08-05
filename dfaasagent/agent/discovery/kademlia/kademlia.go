package kademlia

import (
	"context"
	"github.com/multiformats/go-multiaddr"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/utils/maddrhelp"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/pkg/errors"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
)

// This package handles the Kademlia peer discovery process

type BootstrapConfiguration struct {
	BootstrapNodes      bool    
	PublicBootstrapNodes bool
	BootstrapNodesList  []string
	BootstrapNodesFile  string
	BootstrapForce      bool
}

// kademliaDebugLogging decides wheather to enable or disable logging for this pakcage
const kademliaDebugLogging = true

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
		return errors.Wrap(err, "Error while starting the DHT for Kademlia peer discovery")
	}

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	err = kadDHT.Bootstrap(ctx)
	if err != nil {
		return errors.Wrap(err, "Error while bootstrapping the DHT for Kademlia peer discovery")
	}

	// Let's connect to the bootstrap nodes. They will tell us about the other
	// nodes in the network.

	bootstrapNodes, err := BuildBoostrapNodes(bootstrapConfig)

	var wg sync.WaitGroup
	var chanErrConn = make(chan error, len(bootstrapNodes))
	for _, peerAddr := range bootstrapNodes {
		peerInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			return errors.Wrap(err, "Error while getting information from the bootstrap node's address \""+peerAddr.String()+"\"")
		}

		if kademliaDebugLogging {
			logger.Debug("Kademlia: connecting to bootstrap node \"" + peerAddr.String() + "\"...")
		}
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := p2pHost.Connect(ctx, *peerInfo)
			if err != nil {
				errWrap := errors.Wrap(err, "Connection failed to a bootstrap node (skipping)")

				if bootstrapConfig.BootstrapForce {
					chanErrConn <- errWrap
				} else {
					logger.Error("Kademlia: ", errWrap)
				}

				return
			}

			logger.Info("Kademlia: connection successfully established with bootstrap node \"" + peerInfo.String() + "\"")
		}()
	}
	wg.Wait()
	select {
	case err := <-chanErrConn:
		return err
	default:
	}

	// Announcing ourself on the Kademlia network
	routingDisc := discovery.NewRoutingDiscovery(kadDHT)
	discovery.Advertise(ctx, routingDisc, rendezvous)

	// If everything successful, set the package's static vars
	_ctx = ctx
	_p2pHost = p2pHost
	_rendezvous = rendezvous
	_idleTime = idleTime
	_routingDisc = routingDisc

	return nil
}

// RunDiscovery runs the discovery process. It should run in a goroutine
func RunDiscovery() error {
	logger := logging.Logger()

	for {
		if kademliaDebugLogging {
			logger.Debug("Kademlia: searching for other peers...")
		}

		peerChan, err := _routingDisc.FindPeers(_ctx, _rendezvous)
		if err != nil {
			return errors.Wrap(err, "Error while searching for peers via Kademlia discovery")
		}

		for peerInfo := range peerChan {
			// Ignore ourselves
			if peerInfo.ID == _p2pHost.ID() {
				continue
			}

			if kademliaDebugLogging {
				logger.Debug("Kademlia: found peer \"", peerInfo, "\". Connecting...")
			}
			err := _p2pHost.Connect(_ctx, peerInfo)
			if err != nil {
				errWrap := errors.Wrap(err, "Connection failed to a discovered node (skipping)")
				logger.Error("Kademlia: ", errWrap)
				continue
			}
			if kademliaDebugLogging {
				logger.Debug("Kademlia: connection successful to new discovered node \"", peerInfo, "\"")
			}
		}

		// Now wait a bit and relax...
		time.Sleep(_idleTime)
	}
}

func BuildBoostrapNodes(configuration BootstrapConfiguration) ([]multiaddr.Multiaddr, error) {
	var maddrs []multiaddr.Multiaddr
	var err error

	if configuration.BootstrapNodes {
		if configuration.PublicBootstrapNodes {
			// Use libp2p public bootstrap peers list
			maddrs = dht.DefaultBootstrapPeers
		} else if len(configuration.BootstrapNodesList) > 0 {
			maddrs, err = maddrhelp.StringListToMultiaddrList(configuration.BootstrapNodesList)
			if err != nil {
				return nil, errors.Wrap(err, "Error while converting bootstrap peers string list to multiaddrs list")
			}
		} else if configuration.BootstrapNodesFile != "" {
			maddrs, err = maddrhelp.ParseMAddrFile(configuration.BootstrapNodesFile)
			if err != nil {
				return nil, errors.Wrap(err, "Error while parsing bootstrap peers list from file")
			}
		}
	}
	return maddrs, nil
}
