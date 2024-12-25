//go:build !wasip1

package wnetruntime

import (
	"context"
	"net"
	"net/netip"
	"syscall"
	"unsafe"

	"github.com/egdaemon/wasinet/ffi"
	"github.com/egdaemon/wasinet/ffiguest"
	"github.com/egdaemon/wasinet/internal/langx"
	"golang.org/x/sys/unix"
)

type IP interface {
	Allow(...netip.Prefix) IP
}

type Option func(*network)

func OptionAllow(cidrs ...netip.Prefix) Option {
	return func(s *network) {
		s.allow = append(s.allow, cidrs...)
	}
}

// unrestricted network defaults.
func Unrestricted(opts ...Option) *network {
	return langx.Autoptr(
		langx.Clone(
			network{},
			opts...,
		),
	)
}

// the network by default disallows all network activity. use unrestricted
// or manually configure using options.
func New(opts ...Option) *network {
	return langx.Autoptr(langx.Clone(network{}, opts...))
}

type network struct {
	allow []netip.Prefix
}

func (t network) Open(ctx context.Context, af, socktype, protocol int) (fd int, err error) {
	return unix.Socket(af, socktype, protocol)
}

func (t network) Bind(ctx context.Context, fd int, sa unix.Sockaddr) error {
	return unix.Bind(fd, sa)
}

func (t network) Connect(ctx context.Context, fd int, sa unix.Sockaddr) error {
	return unix.Connect(fd, sa)
}

func (t network) Listen(ctx context.Context, fd, backlog int) error {
	return unix.Listen(fd, backlog)
}

func (t network) LocalAddr(ctx context.Context, fd int) (unix.Sockaddr, error) {
	return unix.Getsockname(fd)
}

func (t network) PeerAddr(ctx context.Context, fd int) (unix.Sockaddr, error) {
	return unix.Getpeername(fd)
}

func (t network) SetSocketOption(ctx context.Context, fd int, level, name int, value []byte) error {
	switch name {
	case syscall.SO_LINGER: // this is untested.
		v := ffiguest.RawRead[unix.Timeval](unsafe.Pointer(&value))
		return unix.SetsockoptTimeval(fd, level, name, v)
	case syscall.SO_BINDTODEVICE: // this is untested.
		return ffi.Errno(unix.SetsockoptString(fd, level, name, string(value)))
	default:
		value := ffiguest.ReadUint32(unsafe.Pointer(&value), uint32(len(value)))
		return ffi.Errno(unix.SetsockoptInt(fd, level, name, int(value)))
	}
}

func (t network) GetSocketOption(ctx context.Context, fd int, level, name int, value []byte) (any, error) {
	switch name {
	case syscall.SO_LINGER:
		return unix.Timeval{}, syscall.ENOTSUP
	case syscall.SO_BINDTODEVICE:
		return "", syscall.ENOTSUP
	default:
		return unix.GetsockoptInt(int(fd), int(level), int(name))
	}
}

func (t network) Shutdown(ctx context.Context, fd, how int) error {
	return unix.Shutdown(fd, how)
}

func (t network) AddrIP(ctx context.Context, network string, address string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, network, address)
}

func (t network) AddrPort(ctx context.Context, network string, service string) (int, error) {
	return net.DefaultResolver.LookupPort(ctx, network, service)
}
