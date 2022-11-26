package netx

import (
	"fmt"
	"net"
)

// NewNoopListener -- junk listener.
func NewNoopListener() net.Listener {
	return noopListener{}
}

type noopListener struct{}

func (noopListener) Accept() (net.Conn, error) {
	return nil, fmt.Errorf("noopListener can not accept connections")
}

func (noopListener) Close() error {
	return nil
}

func (noopListener) Addr() net.Addr {
	return &net.UnixAddr{
		Name: "foo",
		Net:  "unix",
	}
}

func AddrToString(addrs ...*net.TCPAddr) []string {
	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, addr.String())
	}

	return result
}
