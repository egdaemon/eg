//go:build !wasip1

package wasinet

import (
	"log"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/egdaemon/wasinet/wasinet/ffi"
	"golang.org/x/sys/unix"
)

func ReadSockaddr(
	m ffi.Memory, addr unsafe.Pointer, addrlen uint32,
) (unix.Sockaddr, error) {
	var wsa rawsocketaddr
	wsaptr, _ := ffi.Pointer(&wsa)
	if err := ffi.RawRead(m, ffi.Native{}, wsaptr, addr, addrlen); err != nil {
		return nil, err
	}

	return unixsockaddr(wsa)
}

func Sockaddr(sa unix.Sockaddr) (zero rawsocketaddr, error error) {
	switch t := sa.(type) {
	case *unix.SockaddrInet4:
		a := sockipaddr[sockip4]{port: uint32(t.Port), addr: sockip4{ip: t.Addr}}
		return a.sockaddr(), nil

	case *unix.SockaddrInet6:
		a := sockipaddr[sockip6]{port: uint32(t.Port), addr: sockip6{ip: t.Addr, zone: strconv.FormatUint(uint64(t.ZoneId), 10)}}
		return a.sockaddr(), nil
	case *unix.SockaddrUnix:
		name := t.Name
		if len(name) == 0 {
			// For consistency across platforms, replace empty unix socket
			// addresses with @. On Linux, addresses where the first byte is
			// a null byte are considered abstract unix sockets, and the first
			// byte is replaced with @.
			name = "@"
		}
		return (&sockaddrUnix{name: name}).sockaddr(), nil
	default:
		log.Printf("unspoorted unix.Sockaddr: %T\n", t)
		return zero, syscall.EINVAL
	}
}
