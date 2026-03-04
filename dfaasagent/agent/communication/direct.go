// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package communication

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
)

// DirectProtocol is the libp2p protocol ID used for directed dfaas messages.
const DirectProtocol = "/dfaas/msg/1.0.0"

// msgTypeEnvelope is used to peek at the msg_type field before full decoding.
// The msg_type field is nested under "header" in all common vocabulary messages.
type msgTypeEnvelope struct {
	Header struct {
		MsgType string `json:"msg_type"`
	} `json:"header"`
}

// HandlerFunc is called when a directed message of a specific type arrives.
// It receives the raw JSON payload and may return a response object (which will
// be JSON-encoded and sent back) or nil (fire-and-forget, no response sent).
type HandlerFunc func(raw json.RawMessage) (response interface{}, err error)

// DirectMessenger provides directed peer-to-peer messaging over libp2p streams.
// It supports fire-and-forget (Send) and request/response (Request) patterns.
type DirectMessenger interface {
	// PeerID returns the string representation of this messenger's peer ID.
	PeerID() string

	// SetHandler registers a handler for messages of the given type.
	// Replaces any previously registered handler for that type.
	SetHandler(msgType string, h HandlerFunc)

	// Send delivers msg to the peer identified by peerID and returns without
	// waiting for a response (fire-and-forget).
	Send(ctx context.Context, peerID string, msg interface{}) error

	// Request delivers msg to peerID and waits for a response, returning the
	// raw JSON of the response message.
	Request(ctx context.Context, peerID string, msg interface{}) (json.RawMessage, error)
}

// directMessenger is the concrete implementation of DirectMessenger.
type directMessenger struct {
	host     host.Host
	timeout  time.Duration
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

// NewDirectMessenger creates a DirectMessenger backed by h and registers the
// libp2p stream handler for DirectProtocol.
func NewDirectMessenger(h host.Host, timeout time.Duration) DirectMessenger {
	dm := &directMessenger{
		host:     h,
		timeout:  timeout,
		handlers: make(map[string]HandlerFunc),
	}
	h.SetStreamHandler(DirectProtocol, dm.handleStream)
	return dm
}

// PeerID returns the local peer's string ID.
func (dm *directMessenger) PeerID() string {
	return dm.host.ID().String()
}

// SetHandler registers h for the given msgType.
// Safe to call concurrently with incoming stream handlers.
func (dm *directMessenger) SetHandler(msgType string, h HandlerFunc) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.handlers[msgType] = h
}

// Send opens a stream to peerID, writes msg, and closes the stream.
func (dm *directMessenger) Send(ctx context.Context, peerID string, msg interface{}) error {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return errors.Wrap(err, "invalid peer ID")
	}

	ctx, cancel := context.WithTimeout(ctx, dm.timeout)
	defer cancel()

	s, err := dm.host.NewStream(ctx, pid, DirectProtocol)
	if err != nil {
		return errors.Wrap(err, "opening directed stream")
	}
	defer s.Close()

	return writeMsg(s, msg)
}

// Request opens a stream to peerID, writes msg, reads the response, and
// returns the raw JSON response bytes.
func (dm *directMessenger) Request(ctx context.Context, peerID string, msg interface{}) (json.RawMessage, error) {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid peer ID")
	}

	ctx, cancel := context.WithTimeout(ctx, dm.timeout)
	defer cancel()

	s, err := dm.host.NewStream(ctx, pid, DirectProtocol)
	if err != nil {
		return nil, errors.Wrap(err, "opening directed stream")
	}
	defer s.Close()

	if err := writeMsg(s, msg); err != nil {
		return nil, err
	}

	// Signal that we are done writing so the remote can start processing.
	if err := s.CloseWrite(); err != nil {
		return nil, errors.Wrap(err, "closing write side of stream")
	}

	return readMsg(s)
}

// handleStream is the libp2p stream handler for incoming directed messages.
func (dm *directMessenger) handleStream(s network.Stream) {
	defer s.Close()

	raw, err := readMsg(s)
	if err != nil {
		return
	}

	// Peek at the message type.
	var env msgTypeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return
	}

	dm.mu.RLock()
	h, ok := dm.handlers[env.Header.MsgType]
	dm.mu.RUnlock()
	if !ok {
		// No handler registered; ignore the message.
		return
	}

	resp, err := h(raw)
	if err != nil || resp == nil {
		return
	}

	// Write response back on the same stream. Error is intentionally discarded:
	// the requester will time out if the write fails.
	_ = writeMsg(s, resp)
}

// writeMsg serialises msg to JSON and writes it with a 4-byte big-endian
// length prefix.
func writeMsg(w io.Writer, msg interface{}) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "marshalling directed message")
	}

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))

	if _, err := w.Write(lenBuf[:]); err != nil {
		return errors.Wrap(err, "writing message length prefix")
	}
	if _, err := w.Write(payload); err != nil {
		return errors.Wrap(err, "writing message payload")
	}
	return nil
}

// readMsg reads a length-prefixed JSON message from r and returns the raw bytes.
func readMsg(r io.Reader) (json.RawMessage, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, errors.Wrap(err, "reading message length prefix")
	}

	length := binary.BigEndian.Uint32(lenBuf[:])
	if length == 0 {
		return nil, errors.New("received zero-length message")
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, errors.Wrap(err, "reading message payload")
	}

	return json.RawMessage(payload), nil
}

// Ensure directMessenger satisfies the interface at compile time.
var _ DirectMessenger = (*directMessenger)(nil)

// CommonMsgTypes lists the broadcast message types that belong to the common
// vocabulary. Used by the pre-filter in agent.go to identify common messages.
var CommonMsgTypes = map[string]struct{}{
	msgtypes.TypeHeartbeat:     {},
	msgtypes.TypeOverloadAlert: {},
	msgtypes.TypeFunctionEvent: {},
}
