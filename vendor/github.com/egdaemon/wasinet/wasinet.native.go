//go:build !wasip1

package wasinet

import (
	"context"
	"log"
	"net"
	"syscall"
	"unsafe"

	"github.com/egdaemon/wasinet/ffi"
	"github.com/egdaemon/wasinet/internal/errorsx"
	"golang.org/x/sys/unix"
)

// The native implementation ensure the api interopt is correct.

func unixsockaddr(v rawsocketaddr) (sa unix.Sockaddr, err error) {
	wsa, err := rawtosockaddr(&v)
	if err != nil {
		return nil, err
	}

	switch t := wsa.(type) {
	case *sockipaddr[sockip4]:
		return &unix.SockaddrInet4{Port: int(t.port), Addr: t.addr.ip}, nil
	case *sockipaddr[sockip6]:
		return &unix.SockaddrInet6{Port: int(t.port), Addr: t.addr.ip, ZoneId: 0}, nil
	case *sockaddrUnix:
		return &unix.SockaddrUnix{Name: t.name}, nil
	default:
		return nil, syscall.ENOTSUP
	}
}

func sock_open(af int32, socktype int32, proto int32, fd unsafe.Pointer) syscall.Errno {
	_fd, errno := unix.Socket(int(af), int(socktype), int(proto))
	ffi.WriteInt32(fd, int32(_fd))
	return ffi.Errno(errno)
}

func sock_bind(fd int32, addrptr unsafe.Pointer, addrlen uint32) syscall.Errno {
	wsa, err := unixsockaddr(ffi.UnsafeClone[rawsocketaddr](addrptr))
	if err != nil {
		return ffi.Errno(err)
	}

	return ffi.Errno(unix.Bind(int(fd), wsa))
}

func sock_listen(fd int32, backlog int32) syscall.Errno {
	return ffi.Errno(unix.Listen(int(fd), int(backlog)))
}

func sock_connect(fd int32, addr unsafe.Pointer, addrlen uint32) syscall.Errno {
	wsa, err := unixsockaddr(ffi.UnsafeClone[rawsocketaddr](addr))
	if err != nil {
		return ffi.Errno(err)
	}
	return ffi.Errno(unix.Connect(int(fd), wsa))
}

func sock_getsockopt(fd int32, level uint32, name uint32, dst unsafe.Pointer, _ uint32) syscall.Errno {
	switch name {
	default:
		v, err := unix.GetsockoptInt(int(fd), int(level), int(name))
		errorsx.MaybePanic(ffi.Uint32Write(ffi.Native{}, dst, uint32(v)))
		return ffi.Errno(err)
	}
}

func sock_setsockopt(fd int32, level uint32, name uint32, valueptr unsafe.Pointer, valueLen uint32) syscall.Errno {
	switch name {
	case syscall.SO_LINGER: // this is untested.
		value := ffi.UnsafeClone[unix.Timeval](valueptr)
		return ffi.Errno(unix.SetsockoptTimeval(int(fd), int(level), int(name), &value))
	case syscall.SO_BINDTODEVICE: // this is untested.
		value := errorsx.Must(ffi.ReadString(ffi.Native{}, valueptr, uint32(valueLen)))
		return ffi.Errno(unix.SetsockoptString(int(fd), int(level), int(name), value))
	default:
		value := errorsx.Must(ffi.Uint32Read(ffi.Native{}, valueptr, valueLen))
		return ffi.Errno(unix.SetsockoptInt(int(fd), int(level), int(name), int(value)))
	}
}

func sock_getlocaladdr(fd int32, addrptr unsafe.Pointer, addrlen uint32) syscall.Errno {
	sa, err := unix.Getsockname(int(fd))
	if err != nil {
		return ffi.Errno(err)
	}
	addr, err := unixsockaddrToRaw(sa)
	if err != nil {
		return ffi.Errno(err)
	}

	if err = ffi.RawWrite(ffi.Native{}, &addr, addrptr, addrlen); err != nil {
		return ffi.Errno(err)
	}

	return ffi.ErrnoSuccess()
}

func sock_getpeeraddr(fd int32, addrptr unsafe.Pointer, addrlen uint32) syscall.Errno {
	sa, err := unix.Getpeername(int(fd))
	if err != nil {
		return ffi.Errno(err)
	}
	addr, err := unixsockaddrToRaw(sa)
	if err != nil {
		return ffi.Errno(err)
	}

	if err = ffi.RawWrite(ffi.Native{}, &addr, addrptr, addrlen); err != nil {
		return ffi.Errno(err)
	}

	return ffi.ErrnoSuccess()
}

func sock_recv_from(
	fd int32,
	iovs unsafe.Pointer, iovslen uint32,
	addrptr unsafe.Pointer,
	iflags int32,
	nread unsafe.Pointer,
	oflags unsafe.Pointer,
) syscall.Errno {
	vecs := errorsx.Must(ffi.ReadSlice[[]byte](ffi.Native{}, iovs, iovslen))
	for {
		n, _, roflags, sa, err := unix.RecvmsgBuffers(int(fd), vecs, nil, int(iflags))
		switch err {
		case nil:
			// nothing to do.
		case syscall.EINTR, syscall.EWOULDBLOCK:
			continue
		default:
			log.Println("failed", err)
			return ffi.Errno(err)
		}

		addr, err := unixsockaddrToRaw(sa)
		if err != nil {
			log.Println("failed", err)
			return ffi.Errno(err)
		}

		if err := ffi.RawWrite(ffi.Native{}, &addr, addrptr, uint32(unsafe.Sizeof(addr))); err != nil {
			log.Println("failed", err)
			return ffi.Errno(err)
		}

		if err := ffi.Uint32Write(ffi.Native{}, nread, uint32(n)); err != nil {
			log.Println("failed", err)
			return ffi.Errno(err)
		}

		if err := ffi.Uint32Write(ffi.Native{}, oflags, uint32(roflags)); err != nil {
			log.Println("failed", err)
			return ffi.Errno(err)
		}

		return ffi.ErrnoSuccess()
	}
}

func sock_send_to(
	fd int32,
	iovs unsafe.Pointer, iovslen uint32,
	addrptr unsafe.Pointer, _addrlen uint32,
	flags int32,
	nwritten unsafe.Pointer,
) syscall.Errno {
	vec, err := ffi.ReadSlice[ffi.Vector](ffi.Native{}, iovs, iovslen)
	if err != nil {
		return ffi.Errno(err)
	}

	vecs, err := ffi.ReadVector[byte](ffi.Native{}, vec...)
	if err != nil {
		return ffi.Errno(err)
	}

	sa, err := unixsockaddr(ffi.UnsafeClone[rawsocketaddr](addrptr))
	if err != nil {
		return ffi.Errno(err)
	}

	// dispatch-run/wasi-go has linux special cased here.
	// did not faithfully follow it because it might be caused by other complexity.
	// https://github.com/dispatchrun/wasi-go/blob/038d5104aacbb966c25af43797473f03c5da3e4f/systems/unix/system.go#L640
	n, err := unix.SendmsgBuffers(int(fd), vecs, nil, sa, int(flags))

	if err := ffi.Uint32Write(ffi.Native{}, nwritten, uint32(n)); err != nil {
		return ffi.Errno(err)
	}

	return ffi.Errno(err)
}

func sock_shutdown(fd, how int32) syscall.Errno {
	return ffi.Errno(unix.Shutdown(int(fd), int(how)))
}

func sock_getaddrip(
	networkptr unsafe.Pointer, networklen uint32,
	addressptr unsafe.Pointer, addresslen uint32,
	ipres unsafe.Pointer, maxResLen uint32, ipreslen unsafe.Pointer,
) syscall.Errno {
	var (
		err error
		ip  []net.IP
		buf []byte
	)
	network := unsafe.String((*byte)(networkptr), networklen)
	address := unsafe.String((*byte)(addressptr), addresslen)
	if ip, err = net.DefaultResolver.LookupIP(context.Background(), network, address); err != nil {
		log.Println("sock_getaddrip lookup failed", err)
		return syscall.EINVAL
	}

	reslength := len(ip)
	if reslength*net.IPv6len > int(maxResLen) {
		reslength = int(maxResLen / net.IPv6len)
	}

	buf = make([]byte, 0, maxResLen)
	for _, i := range ip[:reslength] {
		buf = append(buf, i.To16()...)
	}

	*(*unsafe.Pointer)(ipres) = unsafe.Pointer(&buf[0])
	*(*uint32)(ipreslen) = uint32(len(buf))

	return 0
}

func sock_getaddrport(
	networkptr unsafe.Pointer, networklen uint32,
	serviceptr unsafe.Pointer, servicelen uint32,
	portptr unsafe.Pointer,
) uint32 {
	var (
		err  error
		port int
	)

	network := errorsx.Must(ffi.ReadString(ffi.Native{}, networkptr, networklen))
	service := errorsx.Must(ffi.ReadString(ffi.Native{}, serviceptr, servicelen))

	if port, err = net.DefaultResolver.LookupPort(context.Background(), network, service); err != nil {
		return uint32(ffi.Errno(err))
	}

	if err = ffi.Uint32Write(ffi.Native{}, portptr, uint32(port)); err != nil {
		return uint32(ffi.Errno(err))
	}

	return 0
}

// passthrough since there is no diffference.
func sock_determine_host_af_family(
	wasi int32,
) int32 {
	return wasi
}

func netaddrfamily(addr net.Addr) int {
	ipfamily := func(ip net.IP) int {
		if ip.To4() == nil {
			return syscall.AF_INET6
		}

		return syscall.AF_INET
	}

	switch a := addr.(type) {
	case *net.IPAddr:
		return ipfamily(a.IP)
	case *net.TCPAddr:
		return ipfamily(a.IP)
	case *net.UDPAddr:
		return ipfamily(a.IP)
	case *net.UnixAddr:
		return syscall.AF_UNIX
	}

	return syscall.AF_INET
}

func rawtosockaddr(rsa *rawsocketaddr) (sockaddr, error) {
	switch rsa.family {
	case syscall.AF_INET:
		addr := (*sockipaddr[sockip4])(unsafe.Pointer(&rsa.addr))
		return addr, nil
	case syscall.AF_INET6:
		addr := (*sockipaddr[sockip6])(unsafe.Pointer(&rsa.addr))
		return addr, nil
	case syscall.AF_UNIX:
		addr := (*sockaddrUnix)(unsafe.Pointer(&rsa.addr))
		return addr, nil
	default:
		return nil, syscall.ENOTSUP
	}
}
