// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This file implements an API to let strategies sending messages to other
// peers.

package communication

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/msgtypes"
)

// DirectProtocol is the custom libp2p protocol ID used for directed dfaas
// messages.
const DirectProtocol = "/dfaas/msg/1.0.0"

// HandlerFunc is called when a directed message of a specific type arrives.
//
// It receives the raw JSON payload and may return a response object (which will
// be JSON-encoded and sent back) or nil (fire-and-forget, no response sent).
type HandlerFunc func(raw json.RawMessage) (response interface{}, err error)

// DirectMessenger provides directed peer-to-peer messaging over libp2p streams.
// It supports fire-and-forget (Send) and request/response (Request) patterns.
type DirectMessenger interface {
	// PeerID returns the string representation of this messenger's peer ID.
	//
	// If the DirectMessenger is created with local host, return the local
	// peer's ID.
	PeerID() string

	// SetHandler registers a handler for messages of the given type. Replaces
	// any previously registered handler for that type. Safe to call
	// concurrently.
	SetHandler(msgType string, handler HandlerFunc)

	// Send delivers msg to the peer identified by peerID and returns without
	// waiting for a response (fire-and-forget).
	Send(ctx context.Context, peerID string, msg interface{}) error

	// Request delivers msg to peerID and waits for a response, returning the
	// raw JSON of the response message.
	Request(ctx context.Context, peerID string, msg interface{}) (json.RawMessage, error)
}

// directMessenger is the concrete implementation of DirectMessenger.
type directMessenger struct {
	host    host.Host
	timeout time.Duration

	// Since the client can call SetHandler at any time, we must protect the
	// concurrent access to the handlers map.
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

func (dm *directMessenger) PeerID() string {
	return dm.host.ID().String()
}

func (dm *directMessenger) SetHandler(msgType string, handler HandlerFunc) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.handlers[msgType] = handler
}

func (dm *directMessenger) Send(ctx context.Context, peerID string, msg interface{}) error {
	// It is a fire-and-forget message: we open a stream to peerID, write
	// msg,and close the stream.
	pid, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	// The timeout is both for opening stream and sending the message.
	ctx, cancel := context.WithTimeout(ctx, dm.timeout)
	defer cancel()

	stream, err := dm.host.NewStream(ctx, pid, DirectProtocol)
	if err != nil {
		return fmt.Errorf("opening directed stream: %w", err)
	}
	defer stream.Close()

	return writeMsg(stream, msg)
}

// Request opens a stream to peerID, writes msg, reads the response, and
// returns the raw JSON response bytes.
func (dm *directMessenger) Request(ctx context.Context, peerID string, msg interface{}) (json.RawMessage, error) {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return nil, fmt.Errorf("invalid peer ID: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, dm.timeout)
	defer cancel()

	s, err := dm.host.NewStream(ctx, pid, DirectProtocol)
	if err != nil {
		return nil, fmt.Errorf("opening directed stream: %w", err)
	}
	defer s.Close()

	if err := writeMsg(s, msg); err != nil {
		return nil, err
	}

	// Signal that we are done writing so the remote can start processing.
	if err := s.CloseWrite(); err != nil {
		return nil, fmt.Errorf("closing write side of stream: %w", err)
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
	var env msgtypes.MsgEnvelope
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
// length prefix in a single Write call.
func writeMsg(w io.Writer, msg interface{}) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshalling directed message: %w", err)
	}

	buf := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(payload)))
	copy(buf[4:], payload)

	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("writing directed message: %w", err)
	}
	return nil
}

// readMsg reads a length-prefixed JSON message from r and returns the raw bytes.
func readMsg(r io.Reader) (json.RawMessage, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("reading message length prefix: %w", err)
	}

	length := binary.BigEndian.Uint32(lenBuf[:])
	if length == 0 {
		return nil, fmt.Errorf("received zero-length message")
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("reading message payload: %w", err)
	}

	return json.RawMessage(payload), nil
}
