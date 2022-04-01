package mdns

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	"github.com/pkg/errors"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
)

// This package handles the mDNS peer discovery process

// mDNSDiscNotifee is a data structure that will be used as a "notifier" for the
// mDNS discovery service. It must implement the discovery.Notifee interface
type mDNSDiscNotifee struct {
	PeerChan chan peer.AddrInfo
}

// HandlePeerFound will be called automatically each time a new peer is found
// trough the mDNS service. This actually implements discovery.Notifee on the
// mDNSDiscNotifee struct
func (n *mDNSDiscNotifee) HandlePeerFound(peerInfo peer.AddrInfo) {
	n.PeerChan <- peerInfo
}

// mdnsDebugLogging decides wheather to enable or disable logging for this pakcage
const mdnsDebugLogging = true

var _ctx context.Context
var _p2pHost host.Host
var _mDNSService discovery.Service

// Initialize initializes the mDNS discovery service
func Initialize(ctx context.Context, p2pHost host.Host, rendezvous string, interval time.Duration) error {
	mDNSService, err := discovery.NewMdnsService(ctx, p2pHost, interval, rendezvous)
	if err != nil {
		return errors.Wrap(err, "Error while initializing the mDNS discovery service")
	}

	// If everything successful, set the package's static vars
	_ctx = ctx
	_p2pHost = p2pHost
	_mDNSService = mDNSService

	return nil
}

// RunDiscovery runs the discovery process. It should run in a goroutine
func RunDiscovery() error {
	logger := logging.Logger()

	notifier := &mDNSDiscNotifee{}
	notifier.PeerChan = make(chan peer.AddrInfo)

	_mDNSService.RegisterNotifee(notifier)

	for peerInfo := range notifier.PeerChan {
		if mdnsDebugLogging {
			logger.Debug("mDNS: found peer \"", peerInfo, "\". Connecting...")
		}
		err := _p2pHost.Connect(_ctx, peerInfo)
		if err != nil {
			errWrap := errors.Wrap(err, "Connection failed to a discovered node (skipping)")
			logger.Error("mDNS: ", errWrap)
			continue
		}
		if mdnsDebugLogging {
			logger.Debug("mDNS: connection successful to new discovered node \"", peerInfo, "\"")
		}
	}

	return nil
}
