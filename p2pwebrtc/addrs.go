package p2pwebrtc

import (
	ma "github.com/multiformats/go-multiaddr"
)

// Addrs rewrites a host's addresses for advertisement: every relayed address (ending in /p2p-circuit)
// is followed by the same address with /webrtc appended, and the bare /webrtc listen address, which
// cannot be dialed, is removed. use it through AddrsFactory.
func Addrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	out := make([]ma.Multiaddr, 0, len(addrs))
	for _, a := range addrs {
		if a.Equal(ListenAddr) {
			continue
		}

		out = append(out, a)

		if len(a) > 0 && a[len(a)-1].Protocol().Code == ma.P_CIRCUIT {
			out = append(out, a.AppendComponent(&webrtcComponent))
		}
	}
	return ma.Unique(out)
}

// AddrsFactory composes Addrs with an existing address factory (nil for none), for libp2p.AddrsFactory.
func AddrsFactory(next func([]ma.Multiaddr) []ma.Multiaddr) func([]ma.Multiaddr) []ma.Multiaddr {
	return func(addrs []ma.Multiaddr) []ma.Multiaddr {
		if next != nil {
			addrs = next(addrs)
		}
		return Addrs(addrs)
	}
}

// SignalingAddrs turns /webrtc addresses of a peer into the addresses its signaling stream is dialed
// on: the /webrtc component and a trailing /p2p/<peer> are removed. other addresses are returned as is.
func SignalingAddrs(addrs ...ma.Multiaddr) []ma.Multiaddr {
	out := make([]ma.Multiaddr, 0, len(addrs))
	for _, a := range addrs {
		if len(a) > 0 && a[len(a)-1].Protocol().Code == ma.P_P2P {
			a = a[:len(a)-1]
		}

		stripped := make(ma.Multiaddr, 0, len(a))
		for _, c := range a {
			if c.Protocol().Code == ma.P_WEBRTC {
				continue
			}
			stripped = append(stripped, c)
		}

		if len(stripped) == 0 {
			continue
		}
		out = append(out, stripped)
	}
	return out
}
