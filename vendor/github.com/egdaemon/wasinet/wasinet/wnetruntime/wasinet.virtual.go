//go:build !wasip1 && !windows

package wnetruntime

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Dialer is the subset of *net.Dialer used to establish outbound connections.
// *net.Dialer satisfies this directly; a VPN/tunnel client can substitute
// its own implementation.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Resolver is the subset of *net.Resolver used for DNS lookups. *net.Resolver
// satisfies this directly; a VPN/tunnel client can substitute its own
// implementation so guest DNS queries transit the tunnel too.
type Resolver interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

// PacketDialer establishes the connectionless (UDP-style) local endpoint used
// for per-call-destination sends/receives. *net.ListenConfig satisfies this
// directly via its ListenPacket method; a VPN/tunnel client can substitute
// its own implementation so connectionless guest UDP transits the tunnel too.
type PacketDialer interface {
	ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error)
}

// Virtual returns a Socket implementation that routes every outbound stream
// connection through dialer, every connectionless UDP send/receive through
// packets, and every DNS lookup through resolver instead of opening real
// kernel sockets, enforcing fw the same way network does. Listen/Accept/Bind
// are not supported. dialer, packets, and resolver must all be non-nil.
func Virtual(dialer Dialer, packets PacketDialer, resolver Resolver, fw Firewall) Socket {
	if dialer == nil {
		panic("wnetruntime: Virtual: dialer must not be nil")
	}
	if packets == nil {
		panic("wnetruntime: Virtual: packets must not be nil")
	}
	if resolver == nil {
		panic("wnetruntime: Virtual: resolver must not be nil")
	}

	return &virtual{
		Firewall: fw,
		dialer:   dialer,
		packets:  packets,
		resolver: resolver,
		conns:    map[int]*vconn{},
	}
}

type virtual struct {
	Firewall
	dialer   Dialer
	packets  PacketDialer
	resolver Resolver

	mu     sync.Mutex
	conns  map[int]*vconn
	nextfd int
}

// vconn is the per-synthetic-fd state: created by Open, then populated
// lazily on first use — Connect for SOCK_STREAM (conn), or the first of
// Connect/SendTo/RecvFrom/LocalAddr for SOCK_DGRAM (pconn) — mirroring
// SOCK_STREAM's deferred dial rather than eagerly opening a resource in Open.
type vconn struct {
	af, socktype, proto int // remembered from Open

	mu    sync.Mutex
	conn  net.Conn       // SOCK_STREAM only: nil until Connect succeeds
	pconn net.PacketConn // SOCK_DGRAM only: nil until first use
	peer  net.Addr       // SOCK_DGRAM only: optional default destination set by Connect
}

func (t *virtual) alloc(af, socktype, proto int) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.nextfd++
	t.conns[t.nextfd] = &vconn{af: af, socktype: socktype, proto: proto}
	return t.nextfd
}

func (t *virtual) lookup(fd int) (*vconn, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	c, ok := t.conns[fd]
	if !ok {
		return nil, unix.EBADF
	}
	return c, nil
}

func (t *virtual) release(fd int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.conns, fd)
}

func (t *virtual) Open(ctx context.Context, af, socktype, protocol int) (int, error) {
	switch af {
	case WASI_AF_INET, WASI_AF_INET6:
	default:
		return -1, unix.EAFNOSUPPORT
	}

	switch socktype {
	case unix.SOCK_STREAM, unix.SOCK_DGRAM:
	default:
		return -1, unix.ESOCKTNOSUPPORT
	}

	return t.alloc(af, socktype, protocol), nil
}

func (t *virtual) Bind(ctx context.Context, fd int, sa unix.Sockaddr) error {
	if _, err := t.lookup(fd); err != nil {
		return err
	}
	return syscall.ENOTSUP
}

func (t *virtual) Connect(ctx context.Context, fd int, sa unix.Sockaddr) error {
	c, err := t.lookup(fd)
	if err != nil {
		return err
	}

	if _, ok := sa.(*unix.SockaddrUnix); ok {
		return unix.EAFNOSUPPORT
	}

	// restriction must run before dialer/packets.ListenPacket is ever
	// touched: a blocked destination must never reach either.
	if err := t.restricted(sa); err != nil {
		return err
	}

	if c.socktype == unix.SOCK_DGRAM {
		if _, err := t.ensurePacketConn(ctx, c); err != nil {
			return err
		}

		addr, err := sockaddrToUDPAddr(sa)
		if err != nil {
			return err
		}

		c.mu.Lock()
		c.peer = addr
		c.mu.Unlock()
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return unix.EISCONN
	}

	address, err := sockaddrToHostPort(sa)
	if err != nil {
		return err
	}

	conn, err := t.dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}

	c.conn = conn
	return nil
}

func (t *virtual) Listen(ctx context.Context, fd, backlog int) error {
	return syscall.ENOTSUP
}

func (t *virtual) Accept(ctx context.Context, fd int) (int, unix.Sockaddr, error) {
	return -1, nil, syscall.ENOTSUP
}

// ensurePacketConn lazily materializes vconn.pconn on first use (Connect,
// SendTo, RecvFrom, or LocalAddr — whichever happens first), mirroring
// SOCK_STREAM's deferred dial rather than Open eagerly reaching out.
func (t *virtual) ensurePacketConn(ctx context.Context, c *vconn) (net.PacketConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pconn != nil {
		return c.pconn, nil
	}

	network := "udp4"
	if c.af == WASI_AF_INET6 {
		network = "udp6"
	}

	pc, err := t.packets.ListenPacket(ctx, network, ":0")
	if err != nil {
		return nil, err
	}

	c.pconn = pc
	return pc, nil
}

func (t *virtual) LocalAddr(ctx context.Context, fd int) (unix.Sockaddr, error) {
	c, err := t.lookup(fd)
	if err != nil {
		return nil, err
	}

	if c.socktype == unix.SOCK_DGRAM {
		pc, err := t.ensurePacketConn(ctx, c)
		if err != nil {
			return nil, err
		}
		return netAddrToSockaddr(pc.LocalAddr())
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil, unix.ENOTCONN
	}
	return netAddrToSockaddr(conn.LocalAddr())
}

func (t *virtual) PeerAddr(ctx context.Context, fd int) (unix.Sockaddr, error) {
	c, err := t.lookup(fd)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.socktype == unix.SOCK_DGRAM {
		if c.peer == nil {
			return nil, unix.ENOTCONN
		}
		return netAddrToSockaddr(c.peer)
	}

	if c.conn == nil {
		return nil, unix.ENOTCONN
	}
	return netAddrToSockaddr(c.conn.RemoteAddr())
}

func (t *virtual) Shutdown(ctx context.Context, fd, how int) error {
	c, err := t.lookup(fd)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.socktype == unix.SOCK_DGRAM {
		if c.pconn == nil {
			return unix.ENOTCONN
		}
		err := c.pconn.Close()
		t.release(fd)
		return err
	}

	if c.conn == nil {
		return unix.ENOTCONN
	}

	switch how {
	case unix.SHUT_RD:
		if cr, ok := c.conn.(interface{ CloseRead() error }); ok {
			return cr.CloseRead()
		}
	case unix.SHUT_WR:
		if cw, ok := c.conn.(interface{ CloseWrite() error }); ok {
			return cw.CloseWrite()
		}
	}

	err = c.conn.Close()
	t.release(fd)
	return err
}

func (t *virtual) SetSocketOption(ctx context.Context, fd int, level, name int, value []byte) error {
	c, err := t.lookup(fd)
	if err != nil {
		return err
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	tc, ok := conn.(*net.TCPConn)
	if !ok || len(value) < 4 {
		return syscall.ENOTSUP
	}

	v := binary.LittleEndian.Uint32(value)

	switch name {
	case syscall.TCP_NODELAY:
		return tc.SetNoDelay(v != 0)
	case syscall.SO_KEEPALIVE:
		return tc.SetKeepAlive(v != 0)
	default:
		return syscall.ENOTSUP
	}
}

func (t *virtual) GetSocketOption(ctx context.Context, fd int, level, name int, value []byte) (any, error) {
	if _, err := t.lookup(fd); err != nil {
		return nil, err
	}
	// net.Conn exposes almost none of these as readable state; don't fake
	// parity with network's raw unix.GetsockoptInt.
	return nil, syscall.ENOTSUP
}

func (t *virtual) AddrIP(ctx context.Context, network string, address string) ([]net.IP, error) {
	hosts, err := t.resolver.LookupHost(ctx, address)
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, 0, len(hosts))
	for _, h := range hosts {
		ip := net.ParseIP(h)
		if ip == nil {
			continue
		}

		switch network {
		case "ip4":
			if ip.To4() == nil {
				continue
			}
		case "ip6":
			if ip.To4() != nil {
				continue
			}
		}

		ips = append(ips, ip)
	}

	return ips, nil
}

func (t *virtual) AddrPort(ctx context.Context, network string, service string) (int, error) {
	return net.DefaultResolver.LookupPort(ctx, network, service)
}

func (t *virtual) RecvFrom(ctx context.Context, fd int, vecs [][]byte, oob []byte, flags int) (int, int, unix.Sockaddr, error) {
	c, err := t.lookup(fd)
	if err != nil {
		return 0, 0, nil, err
	}

	if c.socktype == unix.SOCK_DGRAM {
		pc, err := t.ensurePacketConn(ctx, c)
		if err != nil {
			return 0, 0, nil, err
		}

		buf := vecs[0]
		pc.SetReadDeadline(time.Now().Add(nonblockingPollWindow))
		n, addr, err := pc.ReadFrom(buf)
		pc.SetReadDeadline(time.Time{})
		if err != nil {
			return 0, 0, nil, translateTimeout(err)
		}

		// a failure to translate the peer address doesn't invalidate the
		// n bytes already read — report no sockaddr rather than losing data.
		sa, _ := netAddrToSockaddr(addr)
		return n, 0, sa, nil
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return 0, 0, nil, unix.ENOTCONN
	}

	conn.SetReadDeadline(time.Now().Add(nonblockingPollWindow))
	n, err := conn.Read(vecs[0])
	conn.SetReadDeadline(time.Time{})
	// io.EOF is the peer's orderly shutdown, which recvfrom reports as a successful
	// read of whatever arrived alongside it (0 bytes signals EOF to the guest).
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, 0, nil, translateTimeout(err)
	}

	sa, _ := netAddrToSockaddr(conn.RemoteAddr())
	return n, 0, sa, nil
}

func (t *virtual) SendTo(ctx context.Context, fd int, sa unix.Sockaddr, vecs [][]byte, oob []byte, flags int) (int, error) {
	c, err := t.lookup(fd)
	if err != nil {
		return 0, err
	}

	if c.socktype == unix.SOCK_DGRAM {
		pc, err := t.ensurePacketConn(ctx, c)
		if err != nil {
			return 0, err
		}

		var addr net.Addr
		if sa != nil {
			// UDP is connectionless: unlike Connect's one-time check,
			// policy must be re-enforced on every caller-supplied
			// destination.
			if err := t.restricted(sa); err != nil {
				return 0, err
			}
			addr, err = sockaddrToUDPAddr(sa)
			if err != nil {
				return 0, err
			}
		} else {
			c.mu.Lock()
			addr = c.peer
			c.mu.Unlock()
			if addr == nil {
				return 0, unix.ENOTCONN
			}
		}

		pc.SetWriteDeadline(time.Now().Add(nonblockingPollWindow))
		n, err := pc.WriteTo(vecs[0], addr)
		pc.SetWriteDeadline(time.Time{})
		if err != nil {
			return n, translateTimeout(err)
		}
		return n, nil
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return 0, unix.ENOTCONN
	}

	var written int
	for _, v := range vecs {
		conn.SetWriteDeadline(time.Now().Add(nonblockingPollWindow))
		n, err := conn.Write(v)
		conn.SetWriteDeadline(time.Time{})
		written += n
		if err != nil {
			return written, translateTimeout(err)
		}
	}
	return written, nil
}

// nonblockingPollWindow is the deadline offset used to emulate O_NONBLOCK/
// EAGAIN semantics on top of blocking net.Conn/net.PacketConn reads and
// writes. A deadline of exactly time.Now() (or earlier) is treated by the Go
// runtime as already expired and short-circuits the read/write without ever
// attempting it, even when data/buffer space is immediately available — so a
// true zero-wait poll isn't possible through this API. A small positive
// window gives an already-ready operation a chance to complete while still
// failing fast (as EAGAIN, via translateTimeout) when nothing is ready.
const nonblockingPollWindow = time.Millisecond

// translateTimeout converts the immediate-deadline-poll idiom used to
// emulate O_NONBLOCK/EAGAIN semantics on top of blocking net.Conn/
// net.PacketConn reads and writes.
func translateTimeout(err error) error {
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return unix.EAGAIN
	}
	return err
}

func sockaddrPort(sa unix.Sockaddr) (int, error) {
	switch t := sa.(type) {
	case *unix.SockaddrInet4:
		return t.Port, nil
	case *unix.SockaddrInet6:
		return t.Port, nil
	default:
		return 0, unix.EAFNOSUPPORT
	}
}

func sockaddrToHostPort(sa unix.Sockaddr) (string, error) {
	addr, ok := sockaddrAddr(sa)
	if !ok {
		return "", unix.EAFNOSUPPORT
	}

	port, err := sockaddrPort(sa)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(addr.String(), strconv.Itoa(port)), nil
}

func sockaddrToUDPAddr(sa unix.Sockaddr) (*net.UDPAddr, error) {
	addr, ok := sockaddrAddr(sa)
	if !ok {
		return nil, unix.EAFNOSUPPORT
	}

	port, err := sockaddrPort(sa)
	if err != nil {
		return nil, err
	}

	return &net.UDPAddr{IP: addr.AsSlice(), Port: port}, nil
}

// netAddrToSockaddr translates a net.Addr (as returned by net.Conn/
// net.PacketConn implementations) back to the two concrete unix.Sockaddr
// types wasip1syscall.Sockaddr knows how to round-trip to the wasm guest.
func netAddrToSockaddr(a net.Addr) (unix.Sockaddr, error) {
	var (
		ip   net.IP
		port int
	)

	switch t := a.(type) {
	case *net.TCPAddr:
		ip, port = t.IP, t.Port
	case *net.UDPAddr:
		ip, port = t.IP, t.Port
	default:
		return nil, syscall.ENOTSUP
	}

	if v4 := ip.To4(); v4 != nil {
		return &unix.SockaddrInet4{Port: port, Addr: [4]byte(v4)}, nil
	}

	if v6 := ip.To16(); v6 != nil {
		return &unix.SockaddrInet6{Port: port, Addr: [16]byte(v6)}, nil
	}

	return nil, syscall.ENOTSUP
}
