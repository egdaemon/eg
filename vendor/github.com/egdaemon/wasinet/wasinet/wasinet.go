package wasinet

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/egdaemon/wasinet/wasinet/internal/envx"
)

// Hijack golang's networks net.DefaultResolver and http.DefaultTransport.
//
// the transport timeouts are configurable through the environment, durations use
// time.ParseDuration syntax (e.g. 30s, 2m), a value of 0 disables the timeout:
//
//	WASINET_HIJACK_DIAL_TIMEOUT                (default 10s)
//	WASINET_HIJACK_RESPONSE_HEADER_TIMEOUT     (default 0, no timeout)
//	WASINET_HIJACK_TLS_HANDSHAKE_TIMEOUT       (default 10s)
//	WASINET_HIJACK_IDLE_CONN_TIMEOUT           (default 30s)
func Hijack() {
	net.DefaultResolver.Dial = DialContext
	http.DefaultTransport = transport()
}

// internal only. use at your own risk
func InsecureHTTP() *http.Transport {
	t := transport()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return t
}

func transport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&Dialer{
			Timeout: envx.Duration(10*time.Second, "WASINET_HIJACK_DIAL_TIMEOUT"),
		}).DialContext,
		ForceAttemptHTTP2: true,

		MaxIdleConns:          10,
		ResponseHeaderTimeout: envx.Duration(0, "WASINET_HIJACK_RESPONSE_HEADER_TIMEOUT"),
		IdleConnTimeout:       envx.Duration(30*time.Second, "WASINET_HIJACK_IDLE_CONN_TIMEOUT"),
		TLSHandshakeTimeout:   envx.Duration(10*time.Second, "WASINET_HIJACK_TLS_HANDSHAKE_TIMEOUT"),
		ExpectContinueTimeout: 1 * time.Second,
	}
}

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
