//go:build wasip1

package wasip1net

import (
	"net"
	"net/netip"
	"time"

	"github.com/egdaemon/wasinet/wasinet/stdlib/wasip1syscall"
)

type packetConn struct {
	conn *conn
}

func (c *packetConn) Close() error {
	return c.conn.Close()
}

func (c *packetConn) CloseRead() (err error) {
	return c.conn.fd.closeRead()
}

func (c *packetConn) CloseWrite() (err error) {
	return c.conn.fd.closeWrite()
}

func (c *packetConn) Read(b []byte) (int, error) {
	n, _, err := c.ReadFrom(b)
	return n, err
}

func (c *packetConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	switch c.conn.LocalAddr().(type) {
	case *net.UDPAddr:
		n, _, _, addr, err = c.ReadMsgUDP(b, nil)
	default:
		n, _, _, addr, err = c.ReadMsgUnix(b, nil)
	}
	return
}

func (c *packetConn) ReadMsgUnix(b, oob []byte) (n, oobn, flags int, addr *net.UnixAddr, err error) {
	n, oobn, flags, rsa, err := c.conn.fd.readMsg(b, oob)
	if err != nil {
		return 0, 0, 0, addr, err
	}

	if addr, err = wasip1syscall.NetUnix(rsa); err != nil {
		return 0, 0, 0, addr, err
	}

	return n, oobn, flags, addr, err
}

func (c *packetConn) ReadMsgUDP(b, oob []byte) (n, oobn, flags int, addr *net.UDPAddr, err error) {
	n, oobn, flags, addrPort, err := c.ReadMsgUDPAddrPort(b, oob)
	return n, oobn, flags, net.UDPAddrFromAddrPort(addrPort), err
}

func (c *packetConn) ReadMsgUDPAddrPort(b, oob []byte) (n, oobn, flags int, addrPort netip.AddrPort, err error) {
	n, oobn, flags, rsa, err := c.conn.fd.readMsg(b, oob)
	if err != nil {
		return 0, 0, 0, addrPort, err
	}

	if addrPort, err = wasip1syscall.Netipaddrport(rsa); err != nil {
		return 0, 0, 0, addrPort, err
	}

	return n, oobn, flags, addrPort, err
}

func (c *packetConn) Write(b []byte) (int, error) {
	return c.WriteTo(b, c.conn.RemoteAddr())
}

func (c *packetConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	switch a := addr.(type) {
	case *net.UDPAddr:
		if _, ok := c.conn.LocalAddr().(*net.UDPAddr); ok {
			n, _, err := c.WriteMsgUDP(b, nil, a)
			return n, err
		}
	case *net.UnixAddr:
		if _, ok := c.conn.LocalAddr().(*net.UnixAddr); ok {
			n, _, err := c.WriteMsgUnix(b, nil, a)
			return n, err
		}
	}
	return 0, &net.OpError{
		Op:     "write",
		Net:    c.conn.LocalAddr().Network(),
		Addr:   c.conn.LocalAddr(),
		Source: addr,
		Err:    net.InvalidAddrError("address type mismatch"),
	}
}

func (c *packetConn) WriteMsgUnix(b, oob []byte, addr *net.UnixAddr) (n, oobn int, err error) {
	return c.conn.fd.writeMsg(b, oob, wasip1syscall.NetUnixToRaw(addr))
}

func (c *packetConn) WriteMsgUDP(b, oob []byte, addr *net.UDPAddr) (n, oobn int, err error) {
	return c.WriteMsgUDPAddrPort(b, oob, addr.AddrPort())
}

func (c *packetConn) WriteMsgUDPAddrPort(b, oob []byte, addrPort netip.AddrPort) (n, oobn int, err error) {
	return c.conn.fd.writeMsg(b, oob, wasip1syscall.NetipAddrPortToRaw(c.conn.fd.family, c.conn.fd.sotype, addrPort))
}

func (c *packetConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *packetConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *packetConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *packetConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *packetConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
