package wasinet

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// hijack golang's networks net.DefaultResolver
func Hijack() {
	net.DefaultResolver.Dial = DialContext
	http.DefaultTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&Dialer{
			Timeout: 2 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: true,

		MaxIdleConns:          10,
		ResponseHeaderTimeout: 1 * time.Second,
		IdleConnTimeout:       5 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// internal only. use at your own risk
func InsecureHTTP() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Proxy:           http.ProxyFromEnvironment,
		DialContext: (&Dialer{
			Timeout: 2 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: true,

		MaxIdleConns:          10,
		ResponseHeaderTimeout: 1 * time.Second,
		IdleConnTimeout:       5 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// func netipaddrportToRaw(nap netip.AddrPort) (rawsocketaddr, error) {
// 	if nap.Addr().Is4() || nap.Addr().Is4In6() {
// 		a := sockipaddr[sockip4]{port: uint32(nap.Port()), addr: sockip4{ip: nap.Addr().As4()}}
// 		return a.sockaddr(), nil
// 	} else {
// 		a := sockipaddr[sockip6]{port: uint32(nap.Port()), addr: sockip6{ip: nap.Addr().As16()}}
// 		return a.sockaddr(), nil
// 	}
// }

func netOpErr(op string, addr net.Addr, err error) error {
	if err == nil {
		return nil
	}

	return &net.OpError{
		Op:   op,
		Net:  addr.Network(),
		Addr: addr,
		Err:  err,
	}
}

func unresolvedaddr(network, address string) net.Addr {
	return &unresolvedaddress{network: network, address: address}
}

type unresolvedaddress struct{ network, address string }

func (na *unresolvedaddress) Network() string { return na.network }
func (na *unresolvedaddress) String() string  { return na.address }
