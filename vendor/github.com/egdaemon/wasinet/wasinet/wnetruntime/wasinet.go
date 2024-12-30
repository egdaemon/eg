//go:build !wasip1

package wnetruntime

import (
	"context"
	"net"
	"net/netip"
	"syscall"

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
	Accept(ctx context.Context, fd int) (nfd int, sa unix.Sockaddr, err error)
	LocalAddr(ctx context.Context, fd int) (unix.Sockaddr, error)
	PeerAddr(ctx context.Context, fd int) (unix.Sockaddr, error)
	SetSocketOption(ctx context.Context, fd int, level, name int, value []byte) error
	GetSocketOption(ctx context.Context, fd int, level, name int, value []byte) (any, error)
	Shutdown(ctx context.Context, fd, how int) error
	AddrIP(ctx context.Context, network string, address string) ([]net.IP, error)
	AddrPort(ctx context.Context, network string, service string) (int, error)
	RecvFrom(ctx context.Context, fd int, vecs [][]byte, oob []byte, flags int) (int, int, unix.Sockaddr, error)
	SendTo(ctx context.Context, fd int, sa unix.Sockaddr, vecs [][]byte, oob []byte, flags int) (int, error)
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
	// slog.Log(ctx, slog.LevelDebug, "sock_open", slog.Int("af", af), slog.Int("socktype", socktype), slog.Int("protocol", protocol))
	// syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC are required by golang's runtime for the pollfd to operate correctly.
	// as a result we unconditionally set them here.
	return unix.Socket(af, socktype|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, protocol)
}

func (t network) Bind(ctx context.Context, fd int, sa unix.Sockaddr) error {
	// slog.Log(ctx, slog.LevelDebug, "sock_bind", slog.Int("fd", fd), slog.String("addr", fmt.Sprintf("%v", sa)))
	return unix.Bind(fd, sa)
}

func (t network) Connect(ctx context.Context, fd int, sa unix.Sockaddr) (err error) {
	// slog.Log(ctx, slog.LevelDebug, "sock_connect", slog.Int("fd", fd), slog.String("addr", fmt.Sprintf("%v", sa)))
	return unix.Connect(fd, sa)
}

func (t network) Listen(ctx context.Context, fd, backlog int) error {
	// slog.Log(ctx, slog.LevelDebug, "sock_listen", slog.Int("fd", fd), slog.Int("backlog", backlog))
	return unix.Listen(fd, backlog)
}

func (t network) Accept(ctx context.Context, fd int) (nfd int, sa unix.Sockaddr, err error) {
	// slog.Log(ctx, slog.LevelDebug, "sock_accept", slog.Int("fd", fd))
	return unix.Accept(fd)
}

func (t network) LocalAddr(ctx context.Context, fd int) (unix.Sockaddr, error) {
	// slog.Log(ctx, slog.LevelDebug, "sock_localaddr", slog.Int("fd", fd))
	return unix.Getsockname(fd)
}

func (t network) PeerAddr(ctx context.Context, fd int) (_ unix.Sockaddr, err error) {
	// slog.Log(ctx, slog.LevelDebug, "sock_peeraddr", slog.Int("fd", fd))
	return unix.Getpeername(fd)
}

func (t network) SetSocketOption(ctx context.Context, fd int, level, name int, value []byte) error {
	switch name {
	case syscall.SO_LINGER, syscall.SO_RCVTIMEO, syscall.SO_SNDTIMEO:
		v := &unix.Timeval{}
		vptr, vlen := ffi.Slice(value)
		tvptr, _ := ffi.Pointer(v)
		if err := ffi.RawRead(ffi.Native{}, ffi.Native{}, tvptr, vptr, vlen); err != nil {
			return err
		}
		errno := unix.SetsockoptTimeval(fd, level, name, v)
		// slog.Log(ctx, slog.LevelDebug, "sock_setsockopt_timeval", slog.Int("fd", fd), slog.Int("level", level), slog.Int("name", name), slog.Any("value", v), slog.Int("errno", int(ffierrors.Errno(errno))))
		return errno
	case syscall.SO_BINDTODEVICE: // this is untested.
		value := errorsx.Must(ffi.StringReadNative(ffi.Slice(value)))
		// slog.Log(ctx, slog.LevelDebug, "sock_setsockopt_string", slog.Int("fd", fd), slog.Int("level", level), slog.Int("name", name), slog.String("value", value))
		return unix.SetsockoptString(fd, level, name, string(value))
	default:
		value := errorsx.Must(ffi.Uint32ReadNative(ffi.Slice(value)))
		// slog.Log(ctx, slog.LevelDebug, "sock_setsockopt_int", slog.Int("fd", fd), slog.Int("level", level), slog.Int("name", name), slog.Uint64("value", uint64(value)))
		return unix.SetsockoptInt(fd, level, name, int(value))
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
	// slog.Log(ctx, slog.LevelDebug, "sock_shutdown", slog.Int("fd", fd), slog.Int("how", how))
	return unix.Shutdown(fd, how)
}

func (t network) AddrIP(ctx context.Context, network string, address string) ([]net.IP, error) {
	// slog.Log(ctx, slog.LevelDebug, "sock_getaddrip", slog.String("network", network), slog.String("address", address))
	return net.DefaultResolver.LookupIP(ctx, network, address)
}

func (t network) AddrPort(ctx context.Context, network string, service string) (int, error) {
	// slog.Log(ctx, slog.LevelDebug, "sock_getaddrport", slog.String("network", network), slog.String("service", service))
	return net.DefaultResolver.LookupPort(ctx, network, service)
}

func (t network) RecvFrom(ctx context.Context, fd int, vecs [][]byte, oob []byte, flags int) (int, int, unix.Sockaddr, error) {
	n, _, roflags, sa, err := unix.RecvmsgBuffers(fd, vecs, oob, flags)
	return n, roflags, sa, err
}

func (t network) SendTo(ctx context.Context, fd int, sa unix.Sockaddr, vecs [][]byte, oob []byte, flags int) (int, error) {
	// dispatch-run/wasi-go has linux special cased here.
	// did not faithfully follow it because it might be caused by other complexity.
	// https://github.com/dispatchrun/wasi-go/blob/038d5104aacbb966c25af43797473f03c5da3e4f/systems/unix/system.go#L640
	return unix.SendmsgBuffers(int(fd), vecs, oob, sa, int(flags))
}
