//go:build wasip1

package wasinet

import (
	"log"
	"net"
	"sync"
	"syscall"
	"unsafe"
)

//go:wasmimport wasinet_v0 sock_open
//go:noescape
func sock_open(af int32, socktype int32, proto int32, fd unsafe.Pointer) syscall.Errno

//go:wasmimport wasinet_v0 sock_bind
//go:noescape
func sock_bind(fd int32, addr unsafe.Pointer, addrlen uint32) syscall.Errno

//go:wasmimport wasinet_v0 sock_connect
//go:noescape
func sock_connect(fd int32, addr unsafe.Pointer, addrlen uint32) syscall.Errno

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
	addrptr unsafe.Pointer,
	iflags int32,
	nread unsafe.Pointer,
	oflags unsafe.Pointer,
) syscall.Errno

//go:wasmimport wasinet_v0 sock_send_to
//go:noescape
func sock_send_to(
	fd int32,
	iovs unsafe.Pointer, iovslen uint32,
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

func netaddrfamily(addr net.Addr) int {
	translated := func(v int32) int {
		return int(sock_determine_host_af_family(v))
	}
	ipfamily := func(ip net.IP) int {
		if ip.To4() == nil {
			return translated(syscall.AF_INET6)
		}

		return translated(syscall.AF_INET)
	}

	switch a := addr.(type) {
	case *net.IPAddr:
		return ipfamily(a.IP)
	case *net.TCPAddr:
		return ipfamily(a.IP)
	case *net.UDPAddr:
		return ipfamily(a.IP)
	case *net.UnixAddr:
		return translated(syscall.AF_UNIX)
	}

	return translated(syscall.AF_INET)
}

var (
	maponce     sync.Once
	hostAFINET6 = int32(syscall.AF_INET6)
	hostAFINET  = int32(syscall.AF_INET)
	hostAFUNIX  = int32(syscall.AF_UNIX)
)

func rawtosockaddr(rsa *rawsocketaddr) (sockaddr, error) {
	maponce.Do(func() {
		hostAFINET = sock_determine_host_af_family(hostAFINET)
		hostAFINET6 = sock_determine_host_af_family(hostAFINET6)
		hostAFUNIX = sock_determine_host_af_family(hostAFUNIX)
	})

	switch int32(rsa.family) {
	case hostAFINET:
		addr := (*sockipaddr[sockip4])(unsafe.Pointer(&rsa.addr))
		return addr, nil
	case hostAFINET6:
		addr := (*sockipaddr[sockip6])(unsafe.Pointer(&rsa.addr))
		return addr, nil
	case hostAFUNIX:
		addr := (*sockaddrUnix)(unsafe.Pointer(&rsa.addr))
		return addr, nil
	default:
		log.Println("unable to determine socket family", rsa.family)
		return nil, syscall.ENOTSUP
	}
}
