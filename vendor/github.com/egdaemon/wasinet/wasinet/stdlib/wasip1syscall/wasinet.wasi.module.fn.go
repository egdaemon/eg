//go:build wasip1

package wasip1syscall

import (
	"syscall"
	"unsafe"
)

var (
	afmap = _AFFamilyMap{}
)

func init() {
	afmap.UNSPEC = sock_determine_host_af_family(syscall.AF_UNSPEC)
	afmap.UNIX = sock_determine_host_af_family(syscall.AF_UNIX)
	afmap.INET = sock_determine_host_af_family(syscall.AF_INET)
	afmap.INET6 = sock_determine_host_af_family(syscall.AF_INET6)
}

func AF() _AFFamilyMap {
	return afmap
}

func rawtosockaddr(rsa *RawSocketAddress) (sockaddr, error) {
	switch int32(rsa.Family) {
	case AF().INET:
		addr := (*addressany[addrip4])(unsafe.Pointer(&rsa.Addr))
		return addr, nil
	case AF().INET6:
		addr := (*addressany[addrip6])(unsafe.Pointer(&rsa.Addr))
		return addr, nil
	case AF().UNIX:
		addr := (*addressany[addrunix])(unsafe.Pointer(&rsa.Addr))
		return addr, nil
	default:
		return nil, syscall.ENOTSUP
	}
}

//go:wasmimport wasinet_v0 sock_open
//go:noescape
func sock_open(af int32, socktype int32, proto int32, fd unsafe.Pointer) syscall.Errno

//go:wasmimport wasinet_v0 sock_bind
//go:noescape
func sock_bind(fd int32, addr unsafe.Pointer, addrlen uint32) syscall.Errno

//go:wasmimport wasinet_v0 sock_connect
//go:noescape
func sock_connect(fd int32, addr unsafe.Pointer, addrlen uint32) syscall.Errno

//go:wasmimport wasinet_v0 sock_accept
//go:noescape
func sock_accept(fd int32, nfd unsafe.Pointer, addressptr unsafe.Pointer, addresslen uint32) (errno syscall.Errno)

//go:wasmimport wasinet_v0 sock_listen
//go:noescape
func sock_listen(fd int32, backlog int32) syscall.Errno

//go:wasmimport wasinet_v0 sock_getsockopt
//go:noescape
func sock_getsockopt(fd int32, level uint32, name uint32, value unsafe.Pointer, valueLen uint32) syscall.Errno

//go:wasmimport wasinet_v0 sock_setsockopt
//go:noescape
func sock_setsockopt(fd int32, level uint32, name uint32, value unsafe.Pointer, valueLen uint32) syscall.Errno

//go:wasmimport wasinet_v0 sock_getlocaladdr
//go:noescape
func sock_getlocaladdr(fd int32, addr unsafe.Pointer, addrlen uint32) syscall.Errno

//go:wasmimport wasinet_v0 sock_getpeeraddr
//go:noescape
func sock_getpeeraddr(fd int32, addr unsafe.Pointer, addrlen uint32) syscall.Errno

//go:wasmimport wasinet_v0 sock_recv_from
//go:noescape
func sock_recv_from(
	fd int32,
	iovs unsafe.Pointer, iovslen uint32,
	oob unsafe.Pointer, ooblen uint32,
	addrptr unsafe.Pointer, _addrlen uint32,
	iflags int32,
	nread unsafe.Pointer,
	oflags unsafe.Pointer,
) syscall.Errno

//go:wasmimport wasinet_v0 sock_send_to
//go:noescape
func sock_send_to(
	fd int32,
	iovs unsafe.Pointer, iovslen uint32,
	oob unsafe.Pointer, ooblen uint32,
	addrptr unsafe.Pointer, _addrlen uint32,
	flags int32,
	nwritten unsafe.Pointer,
) syscall.Errno

//go:wasmimport wasinet_v0 sock_shutdown
func sock_shutdown(fd, how int32) syscall.Errno

//go:wasmimport wasinet_v0 sock_getaddrip
//go:noescape
func sock_getaddrip(
	networkptr unsafe.Pointer, networklen uint32,
	addressptr unsafe.Pointer, addresslen uint32,
	ipres unsafe.Pointer, maxResLen uint32, ipreslen unsafe.Pointer,
) syscall.Errno

//go:wasmimport wasinet_v0 sock_getaddrport
//go:noescape
func sock_getaddrport(
	networkptr unsafe.Pointer, networklen uint32,
	serviceptr unsafe.Pointer, servicelen uint32,
	portptr unsafe.Pointer,
) syscall.Errno

//go:wasmimport wasinet_v0 sock_determine_host_af_family
//go:noescape
func sock_determine_host_af_family(
	af int32,
) int32
