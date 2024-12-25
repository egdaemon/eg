//go:build wasip1

package wasinet

import (
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
	addr unsafe.Pointer,
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
