//go:build wasip1

package wasip1net

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"
	"time"
	_ "unsafe"
)

// This helper is implemented in the syscall package. It means we don't have
// to redefine the fd_fdstat_get host import or the fdstat struct it
// populates.
//
// func fd_fdstat_get_type(fd int) (uint8, error)
//
// go:linkname fd_fdstat_get_type syscall.fd_fdstat_get_type
func net_fd_fdstat_get_type(fd int) (uint8, error) {
	return uint8(ftype_socket_stream), nil
}

func fileConnNet(family, sotype int) (string, error) {
	switch family {
	case syscall.AF_UNIX:
		switch sotype {
		case syscall.SOCK_STREAM:
			return "unix", nil
		case syscall.SOCK_DGRAM:
			return "unixgram", nil
		default:
			return "", syscall.ENOTSOCK
		}
	default:
		switch sotype {
		case syscall.SOCK_STREAM:
			return "tcp", nil
		case syscall.SOCK_DGRAM:
			return "udp", nil
		default:
			return "", syscall.ENOTSOCK
		}
	}
}

//go:linkname newFile net.newUnixFile
func newFile(fd int, name string) *os.File

func newFD(family, sotype int, f *os.File, ifn func(*netFD) error) (*netFD, error) {
	net, err := fileConnNet(family, sotype)
	if err != nil {
		return nil, err
	}

	pfd := f.PollFD().Copy()
	fd := newPollFD(net, family, sotype, pfd.Sysfd, &pfd)
	if err := fd.init(ifn); err != nil {
		pfd.Close()
		return nil, err
	}

	return fd, nil
}

func newFileConn(family, sotype int, f *os.File) (net.Conn, error) {
	fd, err := newFD(family, sotype, f, InitConnection)
	if err != nil {
		return nil, err
	}
	return newFdConn(fd)
}

func newFdConn(fd *netFD) (net.Conn, error) {
	switch fd.net {
	case "tcp":
		return &TCPConn{conn{fd: fd}}, nil
	case "udp":
		return &UDPConn{conn{fd: fd}}, nil
	case "unix":
		return &UnixConn{conn{fd: fd}}, nil
	default:
		return nil, fmt.Errorf("unsupported network for file connection: %s", fd.net)
	}
}

func PacketConnFd(family, sotype int, fd uintptr) (net.PacketConn, error) {
	pfd, err := newFD(family, sotype, Socket(fd), InitListener)
	if err != nil {
		return nil, err
	}
	return makePacketConn(&packetConn{conn: &conn{pfd}}), nil
}

func Listener(family, sotype int, fd uintptr) (net.Listener, error) {
	pfd, err := newFD(family, sotype, Socket(fd), InitListener)
	if err != nil {
		return nil, err
	}

	return &listener{pfd}, nil
}

type slistener interface {
	accept() (netfd *netFD, err error)
}

type listener struct{ *netFD }

func (l *listener) Accept() (net.Conn, error) {
	c, err := l.netFD.accept()
	if err != nil {
		return nil, err
	}

	return newFdConn(c)
}

func (l *listener) Addr() net.Addr {
	return l.netFD.LocalAddr()
}

// https://github.com/WebAssembly/WASI/blob/a2b96e81c0586125cc4dc79a5be0b78d9a059925/legacy/preview1/docs.md#filetype

type filetype uint8

const (
	ftype_unknown          filetype = iota // The type of the file descriptor or file is unknown or is different from any of the other types specified.
	ftype_block_device                     // The file descriptor or file refers to a block device inode.
	ftype_character_device                 // The file descriptor or file refers to a character device inode.
	ftype_directory                        // The file descriptor or file refers to a directory inode.
	ftype_regular_file                     // The file descriptor or file refers to a regular file inode.
	ftype_socket_dgram                     // The file descriptor or file refers to a datagram socket.
	ftype_socket_stream                    // The file descriptor or file refers to a byte-stream socket.
	ftype_symbolic_link                    // The file refers to a symbolic link inode.
)

type pollfd interface {
	Accept() (int, syscall.Sockaddr, string, error)
	Close() error
	Dup() (int, string, error)
	Init(net string, pollable bool) error
	Pread(p []byte, off int64) (int, error)
	Pwrite(p []byte, off int64) (int, error)
	RawControl(f func(uintptr)) error
	RawRead(f func(uintptr) bool) error
	RawWrite(f func(uintptr) bool) error
	Read(p []byte) (int, error)
	ReadFrom(p []byte) (int, syscall.Sockaddr, error)
	ReadMsg(p []byte, oob []byte, flags int) (int, int, int, syscall.Sockaddr, error)
	SetBlocking() error
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Shutdown(how int) error
	WaitWrite() error
	Write(p []byte) (int, error)
	WriteMsg(p []byte, oob []byte, sa syscall.Sockaddr) (int, int, error)
	WriteOnce(p []byte) (int, error)
	WriteTo(p []byte, sa syscall.Sockaddr) (int, error)
}

type unknownAddr struct{}

func (unknownAddr) Network() string { return "unknown" }
func (unknownAddr) String() string  { return "unknown" }

// wrapSyscallError takes an error and a syscall name. If the error is
// a syscall.Errno, it wraps it in an os.SyscallError using the syscall name.
func wrapSyscallError(name string, err error) error {
	if _, ok := err.(syscall.Errno); ok {
		err = os.NewSyscallError(name, err)
	}
	return err
}

func setReadBuffer(fd *netFD, bytes int) (err error) {
	log.Println("set read buffer")
	// err := fd.pfd.SetsockoptInt(syscall.SOL_SOCKET, syscall.SO_RCVBUF, bytes)
	runtime.KeepAlive(fd)
	return wrapSyscallError("setsockopt", err)
}

func setWriteBuffer(fd *netFD, bytes int) (err error) {
	log.Println("set write buffer")
	// err := fd.pfd.SetsockoptInt(syscall.SOL_SOCKET, syscall.SO_SNDBUF, bytes)
	runtime.KeepAlive(fd)
	return wrapSyscallError("setsockopt", err)
}

func setKeepAlive(fd *netFD, keepalive bool) (err error) {
	log.Println("set keepalive")
	// err := fd.pfd.SetsockoptInt(syscall.SOL_SOCKET, syscall.SO_KEEPALIVE, boolint(keepalive))
	runtime.KeepAlive(fd)
	return wrapSyscallError("setsockopt", err)
}

func setLinger(fd *netFD, sec int) (err error) {
	log.Println("set linger")
	// var l syscall.Linger
	// if sec >= 0 {
	// 	l.Onoff = 1
	// 	l.Linger = int32(sec)
	// } else {
	// 	l.Onoff = 0
	// 	l.Linger = 0
	// }
	// err := fd.pfd.SetsockoptLinger(syscall.SOL_SOCKET, syscall.SO_LINGER, &l)
	runtime.KeepAlive(fd)
	return wrapSyscallError("setsockopt", err)
}
