// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package p2phostutils

import (
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

// GetConnNodeIDsUniq gets the IDs of the currently connected p2p peers,
// without duplicates
func GetConnNodeIDsUniq(p2pHost host.Host) []peer.ID {
	connections := p2pHost.Network().Conns()
	m := map[peer.ID]bool{}

	for _, conn := range connections {
		m[conn.RemotePeer()] = true
	}

	ids := make([]peer.ID, len(m))

	i := 0
	for id := range m {
		ids[i] = id
		i++
	}

	return ids
}
