package maddrhelp

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/multiformats/go-multiaddr"
)

// This package contains some helper for multiaddresses

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
