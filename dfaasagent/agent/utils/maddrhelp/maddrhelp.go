// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package contains some helper for multiaddresses
package maddrhelp

import (
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/multiformats/go-multiaddr"
)

// BuildHostFullMAddrs given a libp2p Host, returns a list containing full
// multiaddresses that can be used by another agent to reach this host. For
// example, "/ip4/10.0.2.15/tcp/35443/p2p/QmeEe5wmo4Ywi6FvLdbAJdEoV6V4r9tsgMsnTCbMy3gKEm"
// is a full multiaddress because it includes both the ip4/tcp part and the host
// p2p public key.
func BuildHostFullMAddrs(p2pHost host.Host) ([]multiaddr.Multiaddr, error) {
	maddrs := []multiaddr.Multiaddr{}

	addrPart2, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", p2pHost.ID().Pretty()))
	if err != nil {
		return nil, err
	}

	for _, addrPart1 := range p2pHost.Addrs() {
		maddrs = append(maddrs, addrPart1.Encapsulate(addrPart2))
	}

	// Now we can build a full multiaddress to reach the host by encapsulating
	// the two parts, one into the other
	return maddrs, nil
}

func StringListToMultiaddrList(list []string) ([]multiaddr.Multiaddr, error) {
	var maddrs []multiaddr.Multiaddr

	for _, piece := range list {
		piece = strings.TrimSpace(piece)

		if piece == "" {
			continue
		}

		maddr, err := multiaddr.NewMultiaddr(piece)
		if err != nil {
			return nil, err
		}

		maddrs = append(maddrs, maddr)
	}

	return maddrs, nil
}
