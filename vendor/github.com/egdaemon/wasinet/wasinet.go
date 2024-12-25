package wasinet

import (
	"net"
	"net/netip"
)

const (
	Namespace = "wasinet_v0"
)

// hijack the net.DefaultResolver
func Hijack() {
	net.DefaultResolver.Dial = DialContext
}

func netipaddrportToRaw(nap netip.AddrPort) (rawsocketaddr, error) {
	if nap.Addr().Is4() {
		a := sockipaddr[sockip4]{port: uint32(nap.Port()), addr: sockip4{ip: nap.Addr().As4()}}
		return a.sockaddr(), nil
	} else {
		a := sockipaddr[sockip6]{port: uint32(nap.Port()), addr: sockip6{ip: nap.Addr().As16()}}
		return a.sockaddr(), nil
	}
}
