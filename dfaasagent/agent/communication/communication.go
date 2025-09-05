// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package handles the p2p communication with PubSub
package communication

import (
	"context"
	"encoding/json"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/pkg/errors"
)

//////////////////// PUBLIC TYPES ////////////////////

// CBOnReceived represents a callback function called when a pubsub message is
// received from a peer
type CBOnReceived func(msg *pubsub.Message) error

//////////////////// PRIVATE PACKAGE VARS AND INIT FUNCTION ////////////////////

var _ctx context.Context
var _p2pHost host.Host
var _topicName string
var _cbOnReceived CBOnReceived

var _ps *pubsub.PubSub
var _topic *pubsub.Topic
var _subscription *pubsub.Subscription

// Initialize creates the PubSub object and subscribes to the topic.
// The PubSub initialization must be done before the Kademlia one. Otherwise
// the agent won't be able to publish or subscribe.
func Initialize(
	ctx context.Context,
	p2pHost host.Host,
	topicName string,
	cbOnReceived CBOnReceived,
) error {
	// Create a new PubSub object using GossipSub as the router
	ps, err := pubsub.NewGossipSub(ctx, p2pHost)
	if err != nil {
		return errors.Wrap(err, "Error while creating the PubSub object")
	}

	topic, err := ps.Join(topicName)
	if err != nil {
		return errors.Wrap(err, "Error while joining the PubSub topic"+topicName)
	}

	// Subscribe to the PubSub topic
	// All Agents subscribed to the same topc --> broadcasting of messages.
	subscription, err := topic.Subscribe()
	if err != nil {
		return errors.Wrap(err, "Error while subscribing to the PubSub topic")
	}

	// If everything successful, set the package's static vars
	_ctx = ctx
	_p2pHost = p2pHost
	_topicName = topicName
	_cbOnReceived = cbOnReceived

	_ps = ps
	_topic = topic
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

	err = _topic.Publish(_ctx, bytes)
	if err != nil {
		return errors.Wrap(err, "Error while publishing message to PubSub topic"+_topicName)
	}

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
