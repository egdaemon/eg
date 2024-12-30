//go:build !wasip1

package wasip1syscall

import (
	"log"
	"log/slog"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/egdaemon/wasinet/wasinet/ffi"
	"golang.org/x/sys/unix"
)

var (
	afmap = _AFFamilyMap{}
)

func init() {
	afmap.UNSPEC = syscall.AF_UNSPEC
	afmap.UNIX = syscall.AF_UNIX
	afmap.INET = syscall.AF_INET
	afmap.INET6 = syscall.AF_INET6
}

func AF() _AFFamilyMap {
	return afmap
}

func ReadSockaddr(
	m ffi.Memory, addr unsafe.Pointer, addrlen uint32,
) (unix.Sockaddr, error) {
	var wsa RawSocketAddress
	wsaptr, _ := ffi.Pointer(&wsa)
	if err := ffi.RawRead(m, ffi.Native{}, wsaptr, addr, addrlen); err != nil {
		return nil, err
	}

	return UnixSockaddr(wsa)
}

func UnixSockaddr(v RawSocketAddress) (sa unix.Sockaddr, err error) {
	wsa, err := rawtosockaddr(&v)
	if err != nil {
		return nil, err
	}

	switch t := wsa.(type) {
	case *addressany[addrip4]:
		return &unix.SockaddrInet4{Port: int(t.addr.port), Addr: t.addr.ip}, nil
	case *addressany[addrip6]:
		return &unix.SockaddrInet6{Port: int(t.addr.port), Addr: t.addr.ip, ZoneId: 0}, nil
	case *addressany[addrunix]:
		return &unix.SockaddrUnix{Name: t.addr.name}, nil
	default:
		return nil, syscall.ENOTSUP
	}
}

func Sockaddr(sa unix.Sockaddr) (zero *RawSocketAddress, error error) {
	switch t := sa.(type) {
	case *unix.SockaddrInet4:
		a := addressany[addrip4]{
			family: syscall.AF_INET,
			addr:   addrip4{ip: t.Addr, port: uint32(t.Port)},
		}
		return a.Sockaddr(), nil

	case *unix.SockaddrInet6:
		a := addressany[addrip6]{
			family: syscall.AF_INET6,
			addr:   addrip6{ip: t.Addr, port: uint32(t.Port), zone: strconv.FormatUint(uint64(t.ZoneId), 10)},
		}
		return a.Sockaddr(), nil
	case *unix.SockaddrUnix:
		name := t.Name
		if len(name) == 0 {
			// For consistency across platforms, replace empty unix socket
			// addresses with @. On Linux, addresses where the first byte is
			// a null byte are considered abstract unix sockets, and the first
			// byte is replaced with @.
			name = "@"
		}
		return (&addressany[addrunix]{addr: addrunix{name: name}}).Sockaddr(), nil
	default:
		slog.Debug("unsupported unix.Sockaddr", slog.Any("sa", sa))
		return zero, syscall.EINVAL
	}
}

func rawtosockaddr(rsa *RawSocketAddress) (sockaddr, error) {
	log.Println("(native) FAMILY", rsa.Family, syscall.AF_INET, syscall.AF_INET6)
	switch int32(rsa.Family) {
	case syscall.AF_INET:
		addr := (*addressany[addrip4])(unsafe.Pointer(&rsa.Addr))
		return addr, nil
	case syscall.AF_INET6:
		addr := (*addressany[addrip6])(unsafe.Pointer(&rsa.Addr))
		return addr, nil
	case syscall.AF_UNIX:
		addr := (*addressany[addrunix])(unsafe.Pointer(&rsa.Addr))
		return addr, nil
	default:
		log.Println("PANIC")
		return nil, syscall.ENOTSUP
	}
}
