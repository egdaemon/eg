//go:build !wasip1 && !linux && !darwin

package wasip1syscall

import (
	"log"
	"syscall"
	"unsafe"

	"github.com/egdaemon/wasinet/wasinet/ffi"
	"github.com/egdaemon/wasinet/wasinet/ffierrors"
)

type unimplementedsocket struct{}
type NativeSocket = *unimplementedsocket

func ReadSockaddr(
	m ffi.Memory, addr unsafe.Pointer, addrlen uint32,
) (NativeSocket, error) {
	return nil, ffierrors.Errno(syscall.ENOTSUP)
}

func Sockaddr(sa NativeSocket) (zero *RawSocketAddress, error error) {
	log.Println("unsupported unix.Sockaddr", sa)
	return zero, syscall.EINVAL
}

// The native implementation ensure the api interopt is correct.
func sock_open(af int32, socktype int32, proto int32, fd unsafe.Pointer) syscall.Errno {
	log.Println("sock_open", af, socktype, proto)
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_bind(fd int32, addrptr unsafe.Pointer, addrlen uint32) syscall.Errno {
	log.Println("sock_bind", fd)
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_listen(fd int32, backlog int32) syscall.Errno {
	log.Println("sock_listen", fd, backlog)
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_connect(fd int32, addr unsafe.Pointer, addrlen uint32) syscall.Errno {
	log.Println("sock_connect", fd)
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_getsockopt(fd int32, level uint32, name uint32, dst unsafe.Pointer, _ uint32) syscall.Errno {
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_setsockopt(fd int32, level uint32, name uint32, valueptr unsafe.Pointer, valuelen uint32) syscall.Errno {
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_getlocaladdr(fd int32, addrptr unsafe.Pointer, addrlen uint32) syscall.Errno {
	log.Println("sock_localaddr", fd)
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_getpeeraddr(fd int32, addrptr unsafe.Pointer, addrlen uint32) syscall.Errno {
	log.Println("sock_peeraddr", fd)
	return ffierrors.Errno(syscall.ENOTSUP)
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
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_send_to(
	fd int32,
	iovs unsafe.Pointer, iovslen uint32,
	oobptr unsafe.Pointer, ooblen uint32,
	addrptr unsafe.Pointer, _addrlen uint32,
	flags int32,
	nwritten unsafe.Pointer,
) syscall.Errno {
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_shutdown(fd, how int32) syscall.Errno {
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_accept(fd int32, nfd unsafe.Pointer, addressptr unsafe.Pointer, addresslen uint32) (errno syscall.Errno) {
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_getaddrip(
	networkptr unsafe.Pointer, networklen uint32,
	addressptr unsafe.Pointer, addresslen uint32,
	ipres unsafe.Pointer, maxResLen uint32, ipreslen unsafe.Pointer,
) syscall.Errno {
	return ffierrors.Errno(syscall.ENOTSUP)
}

func sock_getaddrport(
	networkptr unsafe.Pointer, networklen uint32,
	serviceptr unsafe.Pointer, servicelen uint32,
	portptr unsafe.Pointer,
) uint32 {
	return 0
}

// passthrough since there is no diffference.
func sock_determine_host_af_family(
	wasi int32,
) int32 {
	return wasi
}

func rawtosockaddr(rsa *RawSocketAddress) (sockaddr, error) {
	return nil, ffierrors.Errno(syscall.ENOTSUP)
}
