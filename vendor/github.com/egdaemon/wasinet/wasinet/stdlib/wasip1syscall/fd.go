package wasip1syscall

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/egdaemon/wasinet/wasinet/ffi"
	"github.com/egdaemon/wasinet/wasinet/ffierrors"
)

func Accept(fd int) (nfd int, addr RawSocketAddress, err error) {
	var (
		_nfd int32
	)
	addrptr, addrlen := ffi.Pointer(&addr)
	nfdptr, _ := ffi.Pointer(&_nfd)
	errno := sock_accept(int32(fd), nfdptr, addrptr, addrlen)
	return int(_nfd), addr, os.NewSyscallError("sock_accept", ffierrors.Error(errno))
}

func RecvFromsingle(fd int, b []byte, oob []byte, flags int32) (n int, addr RawSocketAddress, oflags int32, err error) {
	return recvfrom(fd, [][]byte{b}, oob, flags)
}

func recvfrom(fd int, iovs [][]byte, oob []byte, flags int32) (n int, addr RawSocketAddress, oflags int32, err error) {
	vecs := ffi.VectorSlice(iovs...)
	iovsptr, iovslen := ffi.Slice(vecs)
	oobptr, ooblen := ffi.Slice(oob)
	addrptr, addrlen := ffi.Pointer(&addr)

	errno := sock_recv_from(
		int32(fd),
		iovsptr, iovslen,
		oobptr, ooblen,
		addrptr, addrlen,
		flags,
		unsafe.Pointer(&n),
		unsafe.Pointer(&oflags),
	)

	runtime.KeepAlive(addrptr)
	runtime.KeepAlive(iovsptr)
	runtime.KeepAlive(iovs)
	return n, addr, oflags, os.NewSyscallError("sock_recvfrom", ffierrors.Error(errno))
}

func SendToSingle(fd int, b []byte, oob []byte, addr *RawSocketAddress, flags int32) (int, error) {
	return sendto(fd, [][]byte{b}, oob, addr, flags)
}

func sendto(fd int, iovs [][]byte, oob []byte, addr *RawSocketAddress, flags int32) (int, error) {
	vecs := ffi.VectorSlice(iovs...)
	iovsptr, iovslen := ffi.Slice(vecs)
	oobptr, ooblen := ffi.Slice(oob)
	addrptr, addrlen := ffi.Pointer(addr)

	nwritten := int(0)
	errno := sock_send_to(
		int32(fd),
		iovsptr, iovslen,
		oobptr, ooblen,
		addrptr, addrlen,
		flags,
		unsafe.Pointer(&nwritten),
	)
	runtime.KeepAlive(addr)
	runtime.KeepAlive(iovs)
	return nwritten, ffierrors.Error(errno)
}

func GetsocknameRaw(fd int) (rsa RawSocketAddress, err error) {
	rsaptr, rsalength := ffi.Pointer(&rsa)
	errno := ffierrors.Error(sock_getlocaladdr(int32(fd), rsaptr, rsalength))
	return rsa, os.NewSyscallError("getsockname", errno)
}

func Getsockname(fd int) (sa sockaddr, err error) {
	rsa, err := GetsocknameRaw(fd)
	if err != nil {
		return nil, err
	}
	sa, err = rawtosockaddr(&rsa)
	return sa, os.NewSyscallError("getsockname", err)
}

func getrawpeername(fd int) (rsa RawSocketAddress, err error) {
	rsaptr, rsalength := ffi.Pointer(&rsa)
	errno := sock_getpeeraddr(int32(fd), rsaptr, rsalength)
	return rsa, ffierrors.Error(errno)
}

func Getpeername(fd int) (sockaddr, error) {
	rsa, err := getrawpeername(fd)
	if err != nil {
		return nil, err
	}
	return rawtosockaddr(&rsa)
}

func SetsockoptTimeval(fd int, level uint32, opt uint32, d time.Duration) error {
	type Timeval struct {
		Sec  int64
		Usec int64
	}

	secs := d.Truncate(time.Second)
	milli := d - secs
	tval := &Timeval{Sec: int64(secs / time.Second), Usec: milli.Milliseconds()}
	// chatgpt'd the range for these....
	tval.Usec = max(0, min(tval.Usec, 999999))
	tvalptr, tvallen := ffi.Pointer(tval)
	err := ffierrors.Error(sock_setsockopt(int32(fd), level, opt, tvalptr, tvallen))
	return os.NewSyscallError("setsockopt_timeval", err)
}

func Connect(fd int, rsa *RawSocketAddress) error {
	rawaddr, rawaddrlen := ffi.Pointer(rsa)
	err := ffierrors.Error(sock_connect(int32(fd), rawaddr, rawaddrlen))
	runtime.KeepAlive(rsa)
	return os.NewSyscallError("connect", err)
}

func SetSockoptInt(fd, level, opt int, value int) error {
	var n = int32(value)
	errno := ffierrors.Error(sock_setsockopt(int32(fd), uint32(level), uint32(opt), unsafe.Pointer(&n), 4))
	return os.NewSyscallError("setsockopt_int", errno)
}

func SetReuseAddress(fd int) (err error) {
	if err := SetSockoptInt(fd, SOL_SOCKET, SO_REUSEADDR, 1); err == nil {
		return nil
	}

	// The runtime may not support the option; if that's the case and the
	// address is already in use, binding the socket will fail and we will
	// report the error then.
	switch {
	case errors.Is(err, syscall.ENOPROTOOPT):
	case errors.Is(err, syscall.EINVAL):
	default:
		return err
	}

	return nil
}

func SetSockoptBroadcast(fd int) (err error) {
	if err := SetSockoptInt(fd, SOL_SOCKET, SO_BROADCAST, 1); err == nil {
		return nil
	}

	// If the system does not support broadcast we should still be able
	// to use the datagram socket.
	switch {
	case errors.Is(err, syscall.EINVAL):
	case errors.Is(err, syscall.ENOPROTOOPT):
	default:
		return err
	}

	return nil
}

func GetSockoptInt(fd, level, opt int) (value int, err error) {
	var n int32
	errno := ffierrors.Error(sock_getsockopt(int32(fd), uint32(level), uint32(opt), unsafe.Pointer(&n), 4))
	return int(n), errno
}

func Bind(fd int, rsa *RawSocketAddress) error {
	rawaddr, rawaddrlen := ffi.Pointer(rsa)
	errno := ffierrors.Error(sock_bind(int32(fd), rawaddr, rawaddrlen))
	runtime.KeepAlive(rsa)
	return os.NewSyscallError("sock_bind", errno)
}

func Socket(af, sotype, proto int) (fd int, err error) {
	var newfd int32 = -1
	errno := ffierrors.Error(sock_open(int32(af), int32(sotype), int32(proto), unsafe.Pointer(&newfd)))
	return int(newfd), os.NewSyscallError("socket", errno)
}

func Listen(fd int, backlog int) error {
	return os.NewSyscallError("sock_listen", ffierrors.Error(sock_listen(int32(fd), int32(backlog))))
}

func Shutdown(fd int, how int) error {
	return os.NewSyscallError("sock_shutdown", ffierrors.Error(sock_shutdown(int32(fd), int32(how))))
}
