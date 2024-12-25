package wasinet

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"os"
	"syscall"
	"time"

	"github.com/egdaemon/wasinet/internal/errorsx"
)

// Listen announces on the local network address.
func Listen(network, address string) (net.Listener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6", "unix":
	default:
		return nil, unsupportedNetwork(network, address)
	}
	addrs, err := lookupAddr(context.Background(), oplisten, network, address)
	if err != nil {
		addr := &netAddr{network, address}
		return nil, listenErr(addr, err)
	}

	firstaddr := addrs[0]

	lstn, err := listenAddr(firstaddr)
	return lstn, listenErr(firstaddr, err)
}

// ListenPacket creates a listening packet connection.
func ListenPacket(network, address string) (net.PacketConn, error) {
	switch network {
	case "udp", "udp4", "udp6", "unixgram":
	default:
		return nil, unsupportedNetwork(network, address)
	}
	addrs, err := lookupAddr(context.Background(), oplisten, network, address)
	if err != nil {
		addr := &netAddr{network, address}
		return nil, listenErr(addr, err)
	}
	conn, err := listenPacketAddr(addrs[0])
	if err != nil {
		return nil, listenErr(addrs[0], err)
	}
	return conn, nil
}

func unsupportedNetwork(network, address string) error {
	return fmt.Errorf("unsupported network: %s://%s", network, address)
}

func listenErr(addr net.Addr, err error) error {
	if err == nil {
		return nil
	}
	return newOpError("listen", addr, err)
}

func listenAddr(addr net.Addr) (net.Listener, error) {
	fd, err := socket(netaddrfamily(addr), syscall.SOCK_STREAM, netaddrproto(addr))
	if err != nil {
		return nil, os.NewSyscallError("socket", err)
	}
	defer func() {
		if fd >= 0 {
			syscall.Close(fd)
		}
	}()

	if err := setNonBlock(fd); err != nil {
		return nil, err
	}
	if err := setReuseAddress(fd); err != nil {
		return nil, err
	}

	bindAddr, err := socketAddress(addr)
	if err != nil {
		return nil, os.NewSyscallError("bind", err)
	}

	if err := bind(fd, bindAddr); err != nil {
		return nil, os.NewSyscallError("bind", err)
	}

	const backlog = 64 // TODO: configurable?
	if err := listen(fd, backlog); err != nil {
		return nil, os.NewSyscallError("listen", err)
	}

	name, err := getsockname(fd)
	if err != nil {
		return nil, os.NewSyscallError("getsockname", err)
	}

	f := os.NewFile(uintptr(fd), "")
	fd = -1 // now the *os.File owns the file descriptor
	defer f.Close()

	l, err := net.FileListener(f)
	if err != nil {
		return nil, err
	}
	return makeListener(l, name), nil
}

func listenPacketAddr(addr net.Addr) (net.PacketConn, error) {
	fd, err := socket(netaddrfamily(addr), syscall.SOCK_DGRAM, netaddrproto(addr))
	if err != nil {
		return nil, os.NewSyscallError("socket", err)
	}
	defer func() {
		if fd >= 0 {
			syscall.Close(fd)
		}
	}()

	if err := setNonBlock(fd); err != nil {
		return nil, err
	}
	if err := setReuseAddress(fd); err != nil {
		return nil, err
	}

	bindAddr, err := socketAddress(addr)
	if err != nil {
		return nil, os.NewSyscallError("bind", err)
	}
	if err := bind(fd, bindAddr); err != nil {
		return nil, os.NewSyscallError("bind", err)
	}

	name, err := getsockname(fd)
	if err != nil {
		return nil, os.NewSyscallError("getsockname", err)
	}

	f := os.NewFile(uintptr(fd), "")
	fd = -1 // now the *os.File owns the file descriptor
	return makePacketConn(f, name, nil), nil
}

type listener struct{ net.Listener }

func (l *listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return makeConn(c)
}

type unixListener struct {
	listener
	addr net.UnixAddr
}

func (l *unixListener) Addr() net.Addr {
	return &l.addr
}

func makeListener(l net.Listener, addr sockaddr) net.Listener {
	switch addr.(type) {
	case *sockaddrUnix:
		l = &unixListener{listener: listener{l}}
	default:
		l = &listener{l}
	}
	setNetAddr(syscall.SOCK_STREAM, l.Addr(), addr)
	return l
}

func makePacketConn(f *os.File, laddr, raddr sockaddr) *packetConn {
	conn := &packetConn{file: f}
	if _, unix := laddr.(*sockaddrUnix); unix {
		conn.laddr = new(net.UnixAddr)
		conn.raddr = new(net.UnixAddr)
	} else {
		conn.laddr = new(net.UDPAddr)
		conn.raddr = new(net.UDPAddr)
	}
	setNetAddr(syscall.SOCK_DGRAM, conn.laddr, laddr)
	setNetAddr(syscall.SOCK_DGRAM, conn.raddr, raddr)
	conn.conn, _ = f.SyscallConn()
	return conn
}

type packetConn struct {
	file  *os.File
	laddr net.Addr
	raddr net.Addr
	conn  syscall.RawConn
}

func (c *packetConn) Close() error {
	return c.file.Close()
}

func (c *packetConn) CloseRead() (err error) {
	rawConnErr := c.conn.Control(func(fd uintptr) {
		err = shutdown(int(fd), 1)
	})
	if rawConnErr != nil {
		err = rawConnErr
	}
	return
}

func (c *packetConn) CloseWrite() (err error) {
	rawConnErr := c.conn.Control(func(fd uintptr) {
		err = shutdown(int(fd), 2)
	})
	if rawConnErr != nil {
		err = rawConnErr
	}
	return
}

func (c *packetConn) Read(b []byte) (int, error) {
	n, _, _, _, err := c.ReadMsgUDPAddrPort(b, nil)
	return n, err
}

func (c *packetConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	switch c.laddr.(type) {
	case *net.UDPAddr:
		n, _, _, addr, err = c.ReadMsgUDP(b, nil)
	default:
		n, _, _, addr, err = c.ReadMsgUnix(b, nil)
	}
	return
}

func (c *packetConn) ReadMsgUnix(b, oob []byte) (n, oobn, flags int, addr *net.UnixAddr, err error) {
	rawConnErr := c.conn.Read(func(fd uintptr) (done bool) {
		var raw rawsocketaddr
		var oflags int32
		n, raw, oflags, err = recvfrom(int(fd), [][]byte{b}, 0)
		if err == syscall.EAGAIN {
			return false
		}
		if err == syscall.EINVAL {
			// This error occurs when the socket is shutdown asynchronusly
			// by a call to CloseRead.
			n, err = 0, io.EOF
		} else {
			addr = &net.UnixAddr{
				Net:  "unixgram",
				Name: string(raw.addr[:strlen(raw.addr[:])]),
			}
		}
		flags = int(oflags)
		return true
	})
	if rawConnErr != nil {
		err = rawConnErr
	}
	if n == 0 && err == nil {
		err = io.EOF
	}
	return
}

func (c *packetConn) ReadMsgUDP(b, oob []byte) (n, oobn, flags int, addr *net.UDPAddr, err error) {
	n, oobn, flags, addrPort, err := c.ReadMsgUDPAddrPort(b, oob)
	return n, oobn, flags, net.UDPAddrFromAddrPort(addrPort), err
}

func (c *packetConn) ReadMsgUDPAddrPort(b, oob []byte) (n, oobn, flags int, addrPort netip.AddrPort, err error) {
	werr := c.conn.Read(func(fd uintptr) (done bool) {
		var raw rawsocketaddr
		var oflags int32
		n, raw, oflags, err = recvfrom(int(fd), [][]byte{b}, 0)
		if err == syscall.EAGAIN {
			return false
		}
		if err == syscall.EINVAL {
			// This error occurs when the socket is shutdown asynchronusly
			// by a call to CloseRead.
			n, err = 0, io.EOF
			return true
		}
		if err != nil {
			return true
		}

		sockaddr, err := rawtosockaddr(&raw, 0)
		if err != nil {
			log.Println("failed to decode address", raw.family, syscall.AF_INET, syscall.AF_INET6, err)
			return true
		}

		switch saddr := sockaddr.(type) {
		case *sockipaddr[sockip4]:
			addrPort = netip.AddrPortFrom(netip.AddrFrom4(saddr.addr.ip), uint16(saddr.port))
		case *sockipaddr[sockip6]:
			addrPort = netip.AddrPortFrom(netip.AddrFrom16(saddr.addr.ip), uint16(saddr.port))
		default:
			log.Printf("unsupported address %T\n", saddr)
			return true
		}

		flags = int(oflags)
		return true
	})

	if n == 0 && werr == nil {
		err = io.EOF
	}

	return n, oobn, flags, addrPort, err
}

func (c *packetConn) Write(b []byte) (int, error) {
	return c.WriteTo(b, c.raddr)
}

func (c *packetConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	switch a := addr.(type) {
	case *net.UDPAddr:
		if _, ok := c.laddr.(*net.UDPAddr); ok {
			n, _, err := c.WriteMsgUDP(b, nil, a)
			return n, err
		}
	case *net.UnixAddr:
		if _, ok := c.laddr.(*net.UnixAddr); ok {
			n, _, err := c.WriteMsgUnix(b, nil, a)
			return n, err
		}
	}
	return 0, &net.OpError{
		Op:     "write",
		Net:    c.laddr.Network(),
		Addr:   c.laddr,
		Source: addr,
		Err:    net.InvalidAddrError("address type mismatch"),
	}
}

func (c *packetConn) WriteMsgUnix(b, oob []byte, addr *net.UnixAddr) (n, oobn int, err error) {
	werr := c.conn.Write(func(fd uintptr) (done bool) {
		raw := (&sockaddrUnix{name: addr.Name}).sockaddr()
		n, err = sendto(int(fd), [][]byte{b}, raw, 0)
		return err != syscall.EAGAIN
	})
	return n, oobn, errorsx.Compact(werr, err)
}

func (c *packetConn) WriteMsgUDP(b, oob []byte, addr *net.UDPAddr) (n, oobn int, err error) {
	return c.WriteMsgUDPAddrPort(b, oob, addr.AddrPort())
}

func (c *packetConn) WriteMsgUDPAddrPort(b, oob []byte, addrPort netip.AddrPort) (n, oobn int, err error) {
	rawConnErr := c.conn.Write(func(fd uintptr) (done bool) {
		log.Println("DERP DERP", addrPort.Addr().String(), addrPort.Port())
		var raw rawsocketaddr
		if raw, err = netipaddrportToRaw(addrPort); err != nil {
			return false
		}
		n, err = sendto(int(fd), [][]byte{b}, raw, 0)
		return err != syscall.EAGAIN
	})
	if rawConnErr != nil {
		err = rawConnErr
	}
	return
}

func (c *packetConn) LocalAddr() net.Addr {
	return c.laddr
}

func (c *packetConn) RemoteAddr() net.Addr {
	return c.raddr
}

func (c *packetConn) SetDeadline(t time.Time) error {
	return c.file.SetDeadline(t)
}

func (c *packetConn) SetReadDeadline(t time.Time) error {
	return c.file.SetReadDeadline(t)
}

func (c *packetConn) SetWriteDeadline(t time.Time) error {
	return c.file.SetWriteDeadline(t)
}

func setNetAddr(sotype int, dst net.Addr, src sockaddr) {
	switch a := dst.(type) {
	case *net.IPAddr:
		a.IP, _ = sockaddrIPAndPort(src)
	case *net.TCPAddr:
		a.IP, a.Port = sockaddrIPAndPort(src)
	case *net.UDPAddr:
		a.IP, a.Port = sockaddrIPAndPort(src)
	case *net.UnixAddr:
		switch sotype {
		case syscall.SOCK_STREAM:
			a.Net = "unix"
		case syscall.SOCK_DGRAM:
			a.Net = "unixgram"
		}
		a.Name = sockaddrName(src)
	}
}

// In Go 1.21, the net package cannot initialize the local and remote addresses
// of network connections. For this reason, we use this function to retreive the
// addresses and return a wrapped net.Conn with LocalAddr/RemoteAddr implemented.
func makeConn(c net.Conn) (net.Conn, error) {
	syscallConn, ok := c.(syscall.Conn)
	if !ok {
		return c, nil
	}
	rawConn, err := syscallConn.SyscallConn()
	if err != nil {
		c.Close() // unix.Bind(fd)
		return nil, fmt.Errorf("syscall.Conn.SyscallConn: %w", err)
	}
	rawConnErr := rawConn.Control(func(fd uintptr) {
		var addr sockaddr
		var peer sockaddr

		if addr, err = getsockname(int(fd)); err != nil {
			err = os.NewSyscallError("getsockname", err)
			return
		}

		if peer, err = getpeername(int(fd)); err != nil {
			err = os.NewSyscallError("getpeername", err)
			return
		}

		if _, unix := addr.(*sockaddrUnix); unix {
			c = &unixConn{Conn: c}
		}

		setNetAddr(syscall.SOCK_STREAM, c.LocalAddr(), addr)
		setNetAddr(syscall.SOCK_STREAM, c.RemoteAddr(), peer)

	})
	if err == nil {
		err = rawConnErr
	}
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func sockaddrIPAndPort(addr sockaddr) (net.IP, int) {
	switch a := addr.(type) {
	case *sockipaddr[sockip4]:
		return net.IP(a.addr.ip[:]), int(a.port)
	case *sockipaddr[sockip6]:
		return net.IP(a.addr.ip[:]), int(a.port)
	default:
		return nil, 0
	}
}

func sockaddrName(addr sockaddr) string {
	switch a := addr.(type) {
	case *sockaddrUnix:
		return a.name
	default:
		return ""
	}
}
