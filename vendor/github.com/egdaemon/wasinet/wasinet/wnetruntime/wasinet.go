//go:build !wasip1

package wnetruntime

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"syscall"
	"unsafe"

	"github.com/egdaemon/wasinet/wasinet/ffi"
	"github.com/egdaemon/wasinet/wasinet/internal/errorsx"
	"github.com/egdaemon/wasinet/wasinet/internal/langx"
	"golang.org/x/sys/unix"
)

const (
	Namespace = "wasinet_v0"
)

const (
	WASI_AF_INET  = 2
	WASI_AF_INET6 = 3
)

// Socket interface
type Socket interface {
	Open(ctx context.Context, af, socktype, protocol int) (fd int, err error)
	Bind(ctx context.Context, fd int, sa unix.Sockaddr) error
	Connect(ctx context.Context, fd int, sa unix.Sockaddr) error
	Listen(ctx context.Context, fd, backlog int) error
	LocalAddr(ctx context.Context, fd int) (unix.Sockaddr, error)
	PeerAddr(ctx context.Context, fd int) (unix.Sockaddr, error)
	SetSocketOption(ctx context.Context, fd int, level, name int, value []byte) error
	GetSocketOption(ctx context.Context, fd int, level, name int, value []byte) (any, error)
	Shutdown(ctx context.Context, fd, how int) error
	AddrIP(ctx context.Context, network string, address string) ([]net.IP, error)
	AddrPort(ctx context.Context, network string, service string) (int, error)
	RecvFrom(ctx context.Context, fd int, vecs [][]byte, flags int) (int, int, unix.Sockaddr, error)
	SendTo(ctx context.Context, fd int, sa unix.Sockaddr, vecs [][]byte, flags int) (int, error)
}

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
func Unrestricted(opts ...Option) Socket {
	return langx.Autoptr(
		langx.Clone(
			network{},
			opts...,
		),
	)
}

// the network by default disallows all network activity. use unrestricted
// or manually configure using options.
func New(opts ...Option) Socket {
	return langx.Autoptr(langx.Clone(network{}, opts...))
}

type network struct {
	allow []netip.Prefix
}

func (t network) Open(ctx context.Context, af, socktype, protocol int) (fd int, err error) {
	slog.Log(ctx, slog.LevelDebug, "sock_open", slog.Int("af", af), slog.Int("socktype", socktype), slog.Int("protocol", protocol))
	return unix.Socket(af, socktype, protocol)
}

func (t network) Bind(ctx context.Context, fd int, sa unix.Sockaddr) error {
	slog.Log(ctx, slog.LevelDebug, "sock_bind", slog.Int("fd", fd), slog.String("addr", fmt.Sprintf("%v", sa)))
	return unix.Bind(fd, sa)
}

func (t network) Connect(ctx context.Context, fd int, sa unix.Sockaddr) error {
	slog.Log(ctx, slog.LevelDebug, "sock_connect", slog.Int("fd", fd), slog.String("addr", fmt.Sprintf("%v", sa)))
	return unix.Connect(fd, sa)
}

func (t network) Listen(ctx context.Context, fd, backlog int) error {
	slog.Log(ctx, slog.LevelDebug, "sock_listen", slog.Int("fd", fd), slog.Int("backlog", backlog))
	return unix.Listen(fd, backlog)
}

func (t network) LocalAddr(ctx context.Context, fd int) (unix.Sockaddr, error) {
	slog.Log(ctx, slog.LevelDebug, "sock_localaddr", slog.Int("fd", fd))
	return unix.Getsockname(fd)
}

func (t network) PeerAddr(ctx context.Context, fd int) (unix.Sockaddr, error) {
	slog.Log(ctx, slog.LevelDebug, "sock_peeraddr", slog.Int("fd", fd))
	return unix.Getpeername(fd)
}

func (t network) SetSocketOption(ctx context.Context, fd int, level, name int, value []byte) error {
	switch name {
	case syscall.SO_LINGER: // this is untested.
		v := &unix.Timeval{}
		tvptr, tvlen := ffi.Pointer(v)
		if err := ffi.RawRead(ffi.Native{}, ffi.Native{}, tvptr, unsafe.Pointer(&value), tvlen); err != nil {
			return ffi.Errno(err)
		}
		return unix.SetsockoptTimeval(fd, level, name, v)
	case syscall.SO_BINDTODEVICE: // this is untested.
		value := errorsx.Must(ffi.StringReadNative(ffi.Slice(value)))
		slog.Log(ctx, slog.LevelDebug, "sock_setsockopt_string", slog.Int("fd", fd), slog.Int("level", level), slog.Int("name", name), slog.String("value", value))
		return ffi.Errno(unix.SetsockoptString(fd, level, name, string(value)))
	default:
		value := errorsx.Must(ffi.Uint32ReadNative(ffi.Slice(value)))
		slog.Log(ctx, slog.LevelDebug, "sock_setsockopt_int", slog.Int("fd", fd), slog.Int("level", level), slog.Int("name", name), slog.Uint64("value", uint64(value)))
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
	slog.Log(ctx, slog.LevelDebug, "sock_Shutdown", slog.Int("fd", fd), slog.Int("how", how))
	return unix.Shutdown(fd, how)
}

func (t network) AddrIP(ctx context.Context, network string, address string) ([]net.IP, error) {
	slog.Log(ctx, slog.LevelDebug, "sock_getaddrip", slog.String("network", network), slog.String("address", address))
	return net.DefaultResolver.LookupIP(ctx, network, address)
}

func (t network) AddrPort(ctx context.Context, network string, service string) (int, error) {
	slog.Log(ctx, slog.LevelDebug, "sock_getaddrport", slog.String("network", network), slog.String("service", service))
	return net.DefaultResolver.LookupPort(ctx, network, service)
}

func (t network) RecvFrom(ctx context.Context, fd int, vecs [][]byte, flags int) (int, int, unix.Sockaddr, error) {
	for {
		slog.Log(ctx, slog.LevelDebug, "recvMsgBuffers", slog.Int("fd", fd), slog.Int("flags", flags))
		n, _, roflags, sa, err := unix.RecvmsgBuffers(fd, vecs, nil, flags)
		if err == nil {
			return n, roflags, sa, nil
		}

		switch err {
		case syscall.EINTR, syscall.EWOULDBLOCK:
		default:
			return n, roflags, sa, err
		}

		select {
		case <-ctx.Done():
			return n, roflags, sa, ctx.Err()
		default:
		}
	}
}

func (t network) SendTo(ctx context.Context, fd int, sa unix.Sockaddr, vecs [][]byte, flags int) (int, error) {
	// dispatch-run/wasi-go has linux special cased here.
	// did not faithfully follow it because it might be caused by other complexity.
	// https://github.com/dispatchrun/wasi-go/blob/038d5104aacbb966c25af43797473f03c5da3e4f/systems/unix/system.go#L640
	return unix.SendmsgBuffers(int(fd), vecs, nil, sa, int(flags))
}
