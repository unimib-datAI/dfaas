// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package maddrhelp

import (
	"io/ioutil"
	"strings"

	"github.com/multiformats/go-multiaddr"
)

// ParseMAddrList parses a sep-separated list of multiaddresses
func ParseMAddrList(list, sep string) ([]multiaddr.Multiaddr, error) {
	maddrs := []multiaddr.Multiaddr{}
	pieces := strings.Split(list, sep)

	for _, piece := range pieces {
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

// ParseMAddrComma parses a comma-separated list of multiaddresses
func ParseMAddrComma(list string) ([]multiaddr.Multiaddr, error) {
	return ParseMAddrList(list, ",")
}

// ParseMAddrFile parses a newline-separated list of multiaddresses from a file
func ParseMAddrFile(filepath string) ([]multiaddr.Multiaddr, error) {
	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	// We don't need to remove '\r' characters because in ParseMAddrList we use
	// the strings.TrimSpace function

	return ParseMAddrList(string(bytes), "\n")
}
