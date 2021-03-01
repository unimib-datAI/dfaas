package communication

import (
	"context"
	"encoding/json"

	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/constants"
)

// This package handles the p2p communication with PubSub

//////////////////// PUBLIC TYPES ////////////////////

// CBOnReceived represents a callback function called when a pubsub message is
// received from a peer
type CBOnReceived func(msg *pubsub.Message) error

//////////////////// PRIVATE PACKAGE VARS AND INIT FUNCTION ////////////////////

var _ctx context.Context
var _p2pHost host.Host
var _cbOnReceived CBOnReceived

var _ps *pubsub.PubSub
var _subscription *pubsub.Subscription

// Initialize creates the PubSub object and subscribes to the topic.
// The PubSub initialization must be done before the Kademlia one. Otherwise
// the agent won't be able to publish or subscribe.
func Initialize(
	ctx context.Context,
	p2pHost host.Host,
	cbOnReceived CBOnReceived,
) error {
	// Create a new PubSub object using GossipSub as the router
	ps, err := pubsub.NewGossipSub(ctx, p2pHost)
	if err != nil {
		return errors.Wrap(err, "Error while creating the PubSub object")
	}

	// Subscribe to the PubSub topic
	subscription, err := ps.Subscribe(constants.P2pPubSubTopic)
	if err != nil {
		return errors.Wrap(err, "Error while subscribing to the PubSub topic")
	}

	// If everything successful, set the package's static vars
	_ctx = ctx
	_p2pHost = p2pHost
	_cbOnReceived = cbOnReceived

	_ps = ps
	_subscription = subscription

	return nil
}

//////////////////// OTHER PACKAGE FUNCTIONS ////////////////////

// MarshAndPublish marshals and publishes a message on pubsub
func MarshAndPublish(msg interface{}) error {
	bytes, err := json.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "Error while serializing data structure for publishing")
	}

	_ps.Publish(constants.P2pPubSubTopic, bytes)

	return nil
}

// RunReceiver handles the receiving process. It should run in a goroutine
func RunReceiver() error {
	for {
		msg, err := _subscription.Next(_ctx)
		if err != nil {
			return errors.Wrap(err, "Error while getting a message from the PubSub subscription")
		}

		err = _cbOnReceived(msg)
		if err != nil {
			return errors.Wrap(err, "Error while processing a received PubSub message")
		}
	}
}
