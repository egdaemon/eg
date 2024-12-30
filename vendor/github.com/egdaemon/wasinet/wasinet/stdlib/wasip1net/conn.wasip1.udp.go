//go:build wasip1

package wasip1net

import (
	"net"
	"net/netip"
	"syscall"

	"github.com/egdaemon/wasinet/wasinet/stdlib/wasip1syscall"
)

func ipToSockaddr(family int, ip net.IP, port int, zone string) (syscall.Sockaddr, error) {
	switch family {
	case syscall.AF_INET:
		sa, err := ipToSockaddrInet4(ip, port)
		if err != nil {
			return nil, err
		}
		return &sa, nil
	case syscall.AF_INET6:
		sa, err := ipToSockaddrInet6(ip, port, zone)
		if err != nil {
			return nil, err
		}
		return &sa, nil
	}
	return nil, &net.AddrError{Err: "invalid address family", Addr: ip.String()}
}

func udpAddrfamily(a *net.UDPAddr) int {
	if a == nil || len(a.IP) <= net.IPv4len {
		return syscall.AF_INET
	}
	if a.IP.To4() != nil {
		return syscall.AF_INET
	}
	return syscall.AF_INET6
}

func udpsockaddr(a *net.UDPAddr, family int) (syscall.Sockaddr, error) {
	if a == nil {
		return nil, nil
	}
	return ipToSockaddr(family, a.IP, a.Port, a.Zone)
}

func udpToLocal(a *net.UDPAddr, n string) *net.UDPAddr {
	return &net.UDPAddr{loopbackIP(n), a.Port, a.Zone}
}

func udpOpAddr(a *net.UDPAddr) net.Addr {
	if a == nil {
		return nil
	}
	return a
}

func ipToSockaddrInet4(ip net.IP, port int) (syscall.SockaddrInet4, error) {
	if len(ip) == 0 {
		ip = net.IPv4zero
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return syscall.SockaddrInet4{}, &net.AddrError{Err: "non-IPv4 address", Addr: ip.String()}
	}
	sa := syscall.SockaddrInet4{Port: port}
	copy(sa.Addr[:], ip4)
	return sa, nil
}

func addrPortToSockaddrInet4(ap netip.AddrPort) (syscall.SockaddrInet4, error) {
	// ipToSockaddrInet4 has special handling here for zero length slices.
	// We do not, because netip has no concept of a generic zero IP address.
	addr := ap.Addr()
	if !addr.Is4() {
		return syscall.SockaddrInet4{}, &net.AddrError{Err: "non-IPv4 address", Addr: addr.String()}
	}
	sa := syscall.SockaddrInet4{
		Addr: addr.As4(),
		Port: int(ap.Port()),
	}
	return sa, nil
}

func ipToSockaddrInet6(ip net.IP, port int, zone string) (syscall.SockaddrInet6, error) {
	// In general, an IP wildcard address, which is either
	// "0.0.0.0" or "::", means the entire IP addressing
	// space. For some historical reason, it is used to
	// specify "any available address" on some operations
	// of IP node.
	//
	// When the IP node supports IPv4-mapped IPv6 address,
	// we allow a listener to listen to the wildcard
	// address of both IP addressing spaces by specifying
	// IPv6 wildcard address.
	if len(ip) == 0 || ip.Equal(net.IPv4zero) {
		ip = net.IPv6zero
	}
	// We accept any IPv6 address including IPv4-mapped
	// IPv6 address.
	ip6 := ip.To16()
	if ip6 == nil {
		return syscall.SockaddrInet6{}, &net.AddrError{Err: "non-IPv6 address", Addr: ip.String()}
	}
	sa := syscall.SockaddrInet6{Port: port, ZoneId: uint32(zoneCache.index(zone))}
	copy(sa.Addr[:], ip6)
	return sa, nil
}

func addrPortToSockaddrInet6(ap netip.AddrPort) (syscall.SockaddrInet6, error) {
	// ipToSockaddrInet6 has special handling here for zero length slices.
	// We do not, because netip has no concept of a generic zero IP address.
	//
	// addr is allowed to be an IPv4 address, because As16 will convert it
	// to an IPv4-mapped IPv6 address.
	// The error message is kept consistent with ipToSockaddrInet6.
	addr := ap.Addr()
	if !addr.IsValid() {
		return syscall.SockaddrInet6{}, &net.AddrError{Err: "non-IPv6 address", Addr: addr.String()}
	}
	sa := syscall.SockaddrInet6{
		Addr:   addr.As16(),
		Port:   int(ap.Port()),
		ZoneId: uint32(zoneCache.index(addr.Zone())),
	}
	return sa, nil
}

// An addrPortUDPAddr is a netip.AddrPort-based UDP address that satisfies the Addr interface.
type addrPortUDPAddr struct {
	netip.AddrPort
}

func (addrPortUDPAddr) Network() string { return "udp" }

// UDPConn is the implementation of the Conn and PacketConn interfaces
// for UDP network connections.
type UDPConn struct {
	conn
}

// SyscallConn returns a raw network connection.
// This implements the syscall.Conn interface.
func (c *UDPConn) SyscallConn() (syscall.RawConn, error) {
	if !c.ok() {
		return nil, syscall.EINVAL
	}
	return netsysconn{c.conn}, nil
}

// ReadFromUDP acts like ReadFrom but returns a UDPAddr.
func (c *UDPConn) ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error) {
	// This function is designed to allow the caller to control the lifetime
	// of the returned *net.UDPAddr and thereby prevent an allocation.
	// See https://blog.filippo.io/efficient-go-apis-with-the-inliner/.
	// The real work is done by readFromUDP, below.
	return c.readFromUDP(b, &net.UDPAddr{})
}

// readFromUDP implements ReadFromUDP.
func (c *UDPConn) readFromUDP(b []byte, addr *net.UDPAddr) (int, *net.UDPAddr, error) {
	if !c.ok() {
		return 0, nil, syscall.EINVAL
	}
	n, addr, err := c.readFrom(b, addr)
	if err != nil {
		err = &net.OpError{Op: "read", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return n, addr, err
}

// ReadFrom implements the PacketConn ReadFrom method.
func (c *UDPConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, addr, err := c.readFromUDP(b, &net.UDPAddr{})
	if addr == nil {
		// Return Addr(nil), not Addr(*UDPConn(nil)).
		return n, nil, err
	}
	return n, addr, err
}

// ReadFromUDPAddrPort acts like ReadFrom but returns a netip.AddrPort.
//
// If c is bound to an unspecified address, the returned
// netip.AddrPort's address might be an IPv4-mapped IPv6 address.
// Use netip.Addr.Unmap to get the address without the IPv6 prefix.
func (c *UDPConn) ReadFromUDPAddrPort(b []byte) (n int, addr netip.AddrPort, err error) {
	if !c.ok() {
		return 0, netip.AddrPort{}, syscall.EINVAL
	}
	n, addr, err = c.readFromAddrPort(b)
	if err != nil {
		err = &net.OpError{Op: "read", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return n, addr, err
}

// ReadMsgUDP reads a message from c, copying the payload into b and
// the associated out-of-band data into oob. It returns the number of
// bytes copied into b, the number of bytes copied into oob, the flags
// that were set on the message and the source address of the message.
//
// The packages golang.org/x/net/ipv4 and golang.org/x/net/ipv6 can be
// used to manipulate IP-level socket options in oob.
func (c *UDPConn) ReadMsgUDP(b, oob []byte) (n, oobn, flags int, addr *net.UDPAddr, err error) {
	var ap netip.AddrPort
	n, oobn, flags, ap, err = c.ReadMsgUDPAddrPort(b, oob)
	if ap.IsValid() {
		addr = net.UDPAddrFromAddrPort(ap)
	}
	return
}

// ReadMsgUDPAddrPort is like ReadMsgUDP but returns an netip.AddrPort instead of a UDPAddr.
func (c *UDPConn) ReadMsgUDPAddrPort(b, oob []byte) (n, oobn, flags int, addr netip.AddrPort, err error) {
	if !c.ok() {
		return 0, 0, 0, netip.AddrPort{}, syscall.EINVAL
	}
	n, oobn, flags, addr, err = c.readMsg(b, oob)
	if err != nil {
		err = &net.OpError{Op: "read", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return
}

// WriteToUDP acts like WriteTo but takes a UDPAddr.
func (c *UDPConn) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	if !c.ok() {
		return 0, syscall.EINVAL
	}
	n, err := c.writeTo(b, addr)
	if err != nil {
		err = &net.OpError{Op: "write", Net: c.fd.net, Source: c.fd.laddr, Addr: udpOpAddr(addr), Err: err}
	}
	return n, err
}

// WriteToUDPAddrPort acts like WriteTo but takes a netip.AddrPort.
func (c *UDPConn) WriteToUDPAddrPort(b []byte, addr netip.AddrPort) (int, error) {
	if !c.ok() {
		return 0, syscall.EINVAL
	}
	n, err := c.writeToAddrPort(b, addr)
	if err != nil {
		err = &net.OpError{Op: "write", Net: c.fd.net, Source: c.fd.laddr, Addr: addrPortUDPAddr{addr}, Err: err}
	}
	return n, err
}

// WriteTo implements the PacketConn WriteTo method.
func (c *UDPConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	if !c.ok() {
		return 0, syscall.EINVAL
	}
	a, ok := addr.(*net.UDPAddr)
	if !ok {
		return 0, &net.OpError{Op: "write", Net: c.fd.net, Source: c.fd.laddr, Addr: addr, Err: syscall.EINVAL}
	}
	n, err := c.writeTo(b, a)
	if err != nil {
		err = &net.OpError{Op: "write", Net: c.fd.net, Source: c.fd.laddr, Addr: udpOpAddr(a), Err: err}
	}
	return n, err
}

// WriteMsgUDP writes a message to addr via c if c isn't connected, or
// to c's remote address if c is connected (in which case addr must be
// nil). The payload is copied from b and the associated out-of-band
// data is copied from oob. It returns the number of payload and
// out-of-band bytes written.
//
// The packages golang.org/x/net/ipv4 and golang.org/x/net/ipv6 can be
// used to manipulate IP-level socket options in oob.
func (c *UDPConn) WriteMsgUDP(b, oob []byte, addr *net.UDPAddr) (n, oobn int, err error) {
	if !c.ok() {
		return 0, 0, syscall.EINVAL
	}
	n, oobn, err = c.writeMsg(b, oob, addr)
	if err != nil {
		err = &net.OpError{Op: "write", Net: c.fd.net, Source: c.fd.laddr, Addr: udpOpAddr(addr), Err: err}
	}
	return
}

// WriteMsgUDPAddrPort is like WriteMsgUDP but takes a netip.AddrPort instead of a UDPAddr.
func (c *UDPConn) WriteMsgUDPAddrPort(b, oob []byte, addr netip.AddrPort) (n, oobn int, err error) {
	if !c.ok() {
		return 0, 0, syscall.EINVAL
	}
	n, oobn, err = c.writeMsgAddrPort(b, oob, addr)
	if err != nil {
		err = &net.OpError{Op: "write", Net: c.fd.net, Source: c.fd.laddr, Addr: addrPortUDPAddr{addr}, Err: err}
	}
	return
}

func newUDPConn(fd *netFD) *UDPConn { return &UDPConn{conn{fd}} }

func (c *UDPConn) readFrom(b []byte, addr *net.UDPAddr) (int, *net.UDPAddr, error) {
	n, _, _, rsa, err := c.fd.readMsg(b, nil)
	if err != nil {
		return n, addr, err
	}

	if addr, err = wasip1syscall.UDPAddr(rsa); err != nil {
		return n, addr, err
	}

	return n, addr, err
}

func (c *UDPConn) readFromAddrPort(b []byte) (n int, addr netip.AddrPort, err error) {
	n, _, _, rsa, err := c.fd.readMsg(b, nil)
	if err != nil {
		return n, addr, err
	}

	if addr, err = wasip1syscall.Netipaddrport(rsa); err != nil {
		return n, addr, err
	}

	return n, addr, err
}

func (c *UDPConn) readMsg(b, oob []byte) (n, oobn, flags int, addr netip.AddrPort, err error) {
	n, oobn, oflags, rsa, err := c.fd.readMsg(b, oob)
	if err != nil {
		return n, oobn, flags, addr, err
	}

	if addr, err = wasip1syscall.Netipaddrport(rsa); err != nil {
		return n, oobn, flags, addr, err
	}

	return n, oobn, oflags, addr, nil
}

func (c *UDPConn) writeTo(b []byte, addr *net.UDPAddr) (int, error) {
	if !c.fd.disconnected.Load() {
		return 0, net.ErrWriteToConnected
	}
	if addr == nil {
		return 0, errMissingAddress
	}

	sa, err := wasip1syscall.NetaddrToRaw(c.fd.family, c.fd.sotype, addr)
	if err != nil {
		return 0, err
	}

	return c.fd.writeTo(b, sa)
}

func (c *UDPConn) writeToAddrPort(b []byte, addr netip.AddrPort) (int, error) {
	if !c.fd.disconnected.Load() {
		return 0, net.ErrWriteToConnected
	}
	if !addr.IsValid() {
		return 0, errMissingAddress
	}

	return c.fd.writeTo(b, wasip1syscall.NetipAddrPortToRaw(c.fd.family, c.fd.sotype, addr))
}

func (c *UDPConn) writeMsg(b, oob []byte, addr *net.UDPAddr) (n, oobn int, err error) {
	if !c.fd.disconnected.Load() && addr != nil {
		return 0, 0, net.ErrWriteToConnected
	}
	if !c.fd.disconnected.Load() && addr == nil {
		return 0, 0, errMissingAddress
	}

	sa, err := wasip1syscall.NetaddrToRaw(c.fd.family, c.fd.sotype, addr)
	if err != nil {
		return 0, 0, err
	}

	return c.fd.writeMsg(b, oob, sa)
}

func (c *UDPConn) writeMsgAddrPort(b, oob []byte, addr netip.AddrPort) (n, oobn int, err error) {
	if !c.fd.disconnected.Load() && addr.IsValid() {
		return 0, 0, net.ErrWriteToConnected
	}
	if c.fd.disconnected.Load() && !addr.IsValid() {
		return 0, 0, errMissingAddress
	}

	return c.fd.writeMsg(b, oob, wasip1syscall.NetipAddrPortToRaw(c.fd.family, c.fd.sotype, addr))
}
