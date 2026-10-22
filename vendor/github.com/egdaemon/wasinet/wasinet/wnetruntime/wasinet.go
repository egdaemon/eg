//go:build !wasip1 && !windows

package wnetruntime

import (
	"context"
	"net"
	"net/netip"
	"path/filepath"
	"strings"

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
	Block(...netip.Prefix) IP
}

type FSPrefix struct {
	Host  string
	Guest string
}

type fsremap []FSPrefix

func (t fsremap) Remap(s string) (r string) {
	var (
		best FSPrefix
	)

	for _, m := range t {
		if !strings.HasPrefix(s, m.Guest) {
			continue
		}

		if len(m.Guest) < len(best.Guest) {
			continue
		}

		best = m
	}

	return filepath.Join(best.Host, strings.TrimPrefix(s, best.Guest))
}

type Option func(*network)

// OptionFirewall replaces network's Firewall wholesale with fw. Build fw with
// NewFirewall/FirewallOptionAllow/FirewallOptionBlock, or a plain literal.
func OptionFirewall(fw Firewall) Option {
	return func(n *network) { n.Firewall = fw }
}

func OptionFSPrefixes(prefixes ...FSPrefix) Option {
	return func(n *network) {
		n.fsmap = prefixes
	}
}

// Unrestricted initializes network with an unrestricted firewall: every
// address is permitted unless narrowed via OptionFirewall.
func Unrestricted(opts ...Option) Socket {
	return langx.Autoptr(
		langx.Clone(
			network{Firewall: UnrestrictedFirewall()},
			opts...,
		),
	)
}

// New initializes network with an unrestricted firewall, same as
// Unrestricted; use OptionFirewall to restrict it.
func New(opts ...Option) Socket {
	return langx.Autoptr(langx.Clone(network{Firewall: UnrestrictedFirewall()}, opts...))
}

// PrivatePrefixes are private, loopback, link-local, and multicast address
// space blocked by default by PublicOnly.
func PrivatePrefixes() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("224.0.0.0/4"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("fc00::/7"),
		netip.MustParsePrefix("fe80::/10"),
		netip.MustParsePrefix("ff00::/8"),
	}
}

// PublicOnly restricts network access to public IP address space, blocking
// private, loopback, link-local, and multicast ranges by default. opts are
// applied afterwards and can widen or narrow the defaults.
func PublicOnly(opts ...Option) Socket {
	return langx.Autoptr(
		langx.Clone(
			network{
				Firewall: PublicFirewall(),
			},
			opts...,
		),
	)
}

type network struct {
	Firewall
	fsmap []FSPrefix
}

// sockaddrAddr extracts the IP contained within an inet sockaddr. ok is
// false for sockaddr kinds that don't carry an IP (e.g. unix sockets).
func sockaddrAddr(sa unix.Sockaddr) (addr netip.Addr, ok bool) {
	switch t := sa.(type) {
	case *unix.SockaddrInet4:
		return netip.AddrFrom4(t.Addr), true
	case *unix.SockaddrInet6:
		return netip.AddrFrom16(t.Addr), true
	default:
		return netip.Addr{}, false
	}
}

func (t network) Bind(ctx context.Context, fd int, sa unix.Sockaddr) error {
	// slog.Log(ctx, slog.LevelDebug, "sock_bind", slog.Int("fd", fd), slog.String("addr", fmt.Sprintf("%v", sa)))
	if err := t.restricted(sa); err != nil {
		return err
	}
	return unix.Bind(fd, sa)
}

func (t network) Connect(ctx context.Context, fd int, sa unix.Sockaddr) (err error) {
	switch actual := sa.(type) {
	case *unix.SockaddrUnix:
		remapped := fsremap(t.fsmap).Remap(actual.Name)
		if p, err := unix.Getsockname(fd); err != nil {
			return err
		} else {
			*actual = *p.(*unix.SockaddrUnix)
			actual.Name = remapped
		}

		return unix.Connect(fd, actual)
	default:
		if err := t.restricted(sa); err != nil {
			return err
		}
		// slog.Log(ctx, slog.LevelDebug, "sock_connect", slog.Int("fd", fd), slog.String("addr", fmt.Sprintf("%v", sa)))
		return unix.Connect(fd, sa)
	}
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
	switch sa.(type) {
	case *unix.SockaddrUnix:
		// apparently its fine to send the sock address to a tcp stream
		// but for unix sockets it'll return syscall.EISCONN
		return unix.SendmsgBuffers(int(fd), vecs, oob, nil, int(flags))
	default:
		if err := t.restricted(sa); err != nil {
			return 0, err
		}
		return unix.SendmsgBuffers(int(fd), vecs, oob, sa, int(flags))
	}
}
