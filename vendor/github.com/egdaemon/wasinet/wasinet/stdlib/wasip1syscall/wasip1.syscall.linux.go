//go:build !wasip1 && linux

package wasip1syscall

import (
	"context"
	"log"
	"net"
	"syscall"
	"unsafe"

	"github.com/egdaemon/wasinet/wasinet/ffi"
	"github.com/egdaemon/wasinet/wasinet/ffierrors"
	"github.com/egdaemon/wasinet/wasinet/internal/errorsx"
	"golang.org/x/sys/unix"
)

// The native implementation ensure the api interopt is correct.

func sock_open(af int32, socktype int32, proto int32, fd unsafe.Pointer) syscall.Errno {
	log.Println("sock_open", af, socktype, proto)
	_fd, errno := unix.Socket(int(af), int(socktype), int(proto))
	if err := ffi.Int32Write(ffi.Native{}, fd, int32(_fd)); err != nil {
		return ffierrors.Errno(err)
	}
	return ffierrors.Errno(errno)
}

func sock_bind(fd int32, addrptr unsafe.Pointer, addrlen uint32) syscall.Errno {
	wsa, err := UnixSockaddr(ffi.UnsafeClone[RawSocketAddress](addrptr))
	if err != nil {
		return ffierrors.Errno(err)
	}

	log.Println("sock_bind", fd, wsa)
	return ffierrors.Errno(unix.Bind(int(fd), wsa))
}

func sock_listen(fd int32, backlog int32) syscall.Errno {
	log.Println("sock_listen", fd, backlog)
	return ffierrors.Errno(unix.Listen(int(fd), int(backlog)))
}

func sock_connect(fd int32, addr unsafe.Pointer, addrlen uint32) syscall.Errno {
	wsa, err := UnixSockaddr(ffi.UnsafeClone[RawSocketAddress](addr))
	if err != nil {
		return ffierrors.Errno(err)
	}
	log.Println("sock_connect", fd, wsa)
	return ffierrors.Errno(unix.Connect(int(fd), wsa))
}

func sock_getsockopt(fd int32, level uint32, name uint32, dst unsafe.Pointer, _ uint32) syscall.Errno {
	switch name {
	default:
		v, err := unix.GetsockoptInt(int(fd), int(level), int(name))
		errorsx.MaybePanic(ffi.Uint32Write(ffi.Native{}, dst, uint32(v)))
		return ffierrors.Errno(err)
	}
}

func sock_setsockopt(fd int32, level uint32, name uint32, valueptr unsafe.Pointer, valuelen uint32) syscall.Errno {
	switch name {
	case syscall.SO_LINGER: // this is untested.
		value := ffi.UnsafeClone[unix.Timeval](valueptr)
		return ffierrors.Errno(unix.SetsockoptTimeval(int(fd), int(level), int(name), &value))
	case syscall.SO_BINDTODEVICE: // this is untested.
		value := errorsx.Must(ffi.StringRead(ffi.Native{}, valueptr, uint32(valuelen)))
		return ffierrors.Errno(unix.SetsockoptString(int(fd), int(level), int(name), value))
	default:
		value := errorsx.Must(ffi.Uint32Read(ffi.Native{}, valueptr, valuelen))
		log.Println("sock_setsockopt", fd, level, name, value)
		return ffierrors.Errno(unix.SetsockoptInt(int(fd), int(level), int(name), int(value)))
	}
}

func sock_getlocaladdr(fd int32, addrptr unsafe.Pointer, addrlen uint32) syscall.Errno {
	log.Println("sock_localaddr", fd)
	sa, err := unix.Getsockname(int(fd))
	if err != nil {
		return ffierrors.Errno(err)
	}
	addr, err := Sockaddr(sa)
	if err != nil {
		return ffierrors.Errno(err)
	}

	if err = ffi.RawWrite(ffi.Native{}, &addr, addrptr, addrlen); err != nil {
		return ffierrors.Errno(err)
	}

	return ffierrors.ErrnoSuccess()
}

func sock_getpeeraddr(fd int32, addrptr unsafe.Pointer, addrlen uint32) syscall.Errno {
	log.Println("sock_peeraddr", fd)
	sa, err := unix.Getpeername(int(fd))
	if err != nil {
		return ffierrors.Errno(err)
	}
	addr, err := Sockaddr(sa)
	if err != nil {
		return ffierrors.Errno(err)
	}

	if err = ffi.RawWrite(ffi.Native{}, &addr, addrptr, addrlen); err != nil {
		return ffierrors.Errno(err)
	}

	return ffierrors.ErrnoSuccess()
}

func sock_recv_from(
	fd int32,
	iovs unsafe.Pointer, iovslen uint32,
	oobptr unsafe.Pointer, ooblen uint32,
	addrptr unsafe.Pointer, _addrlen uint32,
	iflags int32,
	nread unsafe.Pointer,
	oflags unsafe.Pointer,
) syscall.Errno {
	oob := errorsx.Must(ffi.BytesRead(ffi.Native{}, oobptr, ooblen))
	vecs := errorsx.Must(ffi.SliceRead[[]byte](ffi.Native{}, iovs, iovslen))
	for {
		n, _, roflags, sa, err := unix.RecvmsgBuffers(int(fd), vecs, oob, int(iflags))
		switch err {
		case nil:
			// nothing to do.
		case syscall.EINTR, syscall.EWOULDBLOCK:
			continue
		default:
			return ffierrors.Errno(err)
		}

		if sa != nil {
			addr, err := Sockaddr(sa)
			if err != nil {
				return ffierrors.Errno(err)
			}

			if err := ffi.RawWrite(ffi.Native{}, &addr, addrptr, uint32(unsafe.Sizeof(addr))); err != nil {
				return ffierrors.Errno(err)
			}
		}

		if err := ffi.Uint32Write(ffi.Native{}, nread, uint32(n)); err != nil {
			return ffierrors.Errno(err)
		}

		if err := ffi.Uint32Write(ffi.Native{}, oflags, uint32(roflags)); err != nil {
			return ffierrors.Errno(err)
		}

		return ffierrors.ErrnoSuccess()
	}
}

func sock_send_to(
	fd int32,
	iovs unsafe.Pointer, iovslen uint32,
	oobptr unsafe.Pointer, ooblen uint32,
	addrptr unsafe.Pointer, _addrlen uint32,
	flags int32,
	nwritten unsafe.Pointer,
) syscall.Errno {
	oob := errorsx.Must(ffi.BytesRead(ffi.Native{}, oobptr, ooblen))
	vec, err := ffi.SliceRead[ffi.Vector](ffi.Native{}, iovs, iovslen)
	if err != nil {
		return ffierrors.Errno(err)
	}

	vecs, err := ffi.VectorRead[byte](ffi.Native{}, vec...)
	if err != nil {
		return ffierrors.Errno(err)
	}

	sa, err := UnixSockaddr(ffi.UnsafeClone[RawSocketAddress](addrptr))
	if err != nil {
		return ffierrors.Errno(err)
	}

	// dispatch-run/wasi-go has linux special cased here.
	// did not faithfully follow it because it might be caused by other complexity.
	// https://github.com/dispatchrun/wasi-go/blob/038d5104aacbb966c25af43797473f03c5da3e4f/systems/unix/system.go#L640
	n, err := unix.SendmsgBuffers(int(fd), vecs, oob, sa, int(flags))

	if err := ffi.Uint32Write(ffi.Native{}, nwritten, uint32(n)); err != nil {
		return ffierrors.Errno(err)
	}

	return ffierrors.Errno(err)
}

func sock_shutdown(fd, how int32) syscall.Errno {
	return ffierrors.Errno(unix.Shutdown(int(fd), int(how)))
}

func sock_accept(fd int32, nfd unsafe.Pointer, addressptr unsafe.Pointer, addresslen uint32) (errno syscall.Errno) {
	_nfd, sa, err := unix.Accept(int(fd))
	if err != nil {
		return ffierrors.Errno(err)
	}

	rsa, err := Sockaddr(sa)
	if err != nil {
		return ffierrors.Errno(err)
	}
	if err = ffi.RawWrite(ffi.Native{}, rsa, addressptr, addresslen); err != nil {
		return ffierrors.Errno(err)
	}

	if err = ffi.Int32Write(ffi.Native{}, nfd, int32(_nfd)); err != nil {
		return ffierrors.Errno(err)
	}

	return ffierrors.Errno(nil)
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

	network := errorsx.Must(ffi.StringRead(ffi.Native{}, networkptr, networklen))
	address := errorsx.Must(ffi.StringRead(ffi.Native{}, addressptr, addresslen))
	if ip, err = net.DefaultResolver.LookupIP(context.Background(), network, address); err != nil {
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

	network := errorsx.Must(ffi.StringRead(ffi.Native{}, networkptr, networklen))
	service := errorsx.Must(ffi.StringRead(ffi.Native{}, serviceptr, servicelen))

	log.Println("sock_getaddrport", network, service)
	if port, err = net.DefaultResolver.LookupPort(context.Background(), network, service); err != nil {
		return uint32(ffierrors.Errno(err))
	}

	if err = ffi.Uint32Write(ffi.Native{}, portptr, uint32(port)); err != nil {
		return uint32(ffierrors.Errno(err))
	}

	return 0
}

// passthrough since there is no diffference.
func sock_determine_host_af_family(
	wasi int32,
) int32 {
	return wasi
}
