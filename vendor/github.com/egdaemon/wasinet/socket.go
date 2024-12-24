package wasinet

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/egdaemon/wasinet/ffiguest"
)

const (
	oplisten = "listen"
	opdial   = "dial"
)

func lookupAddr(_ context.Context, op, network, address string) ([]net.Addr, error) {
	switch network {
	case "unix", "unixgram":
		return []net.Addr{&net.UnixAddr{Name: address, Net: network}}, nil
	default:
	}

	hostname, service, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	port, err := resolveport(network, service)
	if err != nil {

		return nil, os.NewSyscallError("resolveport", err)
	}

	ips, err := resolveaddrip(op, network, hostname)
	if err != nil {

		return nil, os.NewSyscallError("resolveaddrip", err)
	}

	addrs := make([]net.Addr, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, netaddr(network, ip, port))
	}

	if len(addrs) == 0 {
		return nil, &net.DNSError{
			Err:        "lookup failed",
			Name:       hostname,
			IsNotFound: true,
		}
	}

	return addrs, nil
}

func socket(af, sotype, proto int) (fd int, err error) {
	var newfd int32 = -1
	errno := sock_open(int32(af), int32(sotype), int32(proto), unsafe.Pointer(&newfd))
	if errno != 0 {
		return -1, errno
	}
	return int(newfd), nil
}

func bind(fd int, sa sockaddr) error {
	rawaddr, rawaddrlen := ffiguest.Raw(sa.sockaddr())
	errno := sock_bind(int32(fd), rawaddr, rawaddrlen)
	runtime.KeepAlive(sa)
	if errno != 0 {
		return errno
	}
	return nil
}

func listen(fd int, backlog int) error {
	if errno := sock_listen(int32(fd), int32(backlog)); errno != 0 {
		return errno
	}
	return nil
}

func connect(fd int, sa sockaddr) error {
	rawaddr, rawaddrlen := ffiguest.Raw(sa.sockaddr())
	errno := sock_connect(int32(fd), rawaddr, rawaddrlen)
	runtime.KeepAlive(sa)
	if errno != 0 {
		return errno
	}
	return nil
}

func shutdown(fd, how int) error {
	if errno := sock_shutdown(int32(fd), int32(how)); errno != 0 {
		return errno
	}
	return nil
}

func getsockopt(fd, level, opt int) (value int, err error) {
	var n int32
	errno := sock_getsockopt(int32(fd), uint32(level), uint32(opt), unsafe.Pointer(&n), 4)
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}

func setsockopt(fd, level, opt int, value int) error {
	var n = int32(value)
	errno := sock_setsockopt(int32(fd), uint32(level), uint32(opt), unsafe.Pointer(&n), 4)
	if errno != 0 {
		return errno
	}
	return nil
}

func getsockname(fd int) (sa sockaddr, err error) {
	var rsa rawSockaddrAny
	errno := sock_getlocaladdr(int32(fd), unsafe.Pointer(&rsa))
	if errno != 0 {
		return nil, errno
	}
	return anyToSockaddr(&rsa)
}

func getpeername(fd int) (sockaddr, error) {
	var rsa rawSockaddrAny
	errno := sock_getpeeraddr(int32(fd), unsafe.Pointer(&rsa))
	if errno != 0 {
		return nil, errno
	}
	return anyToSockaddr(&rsa)
}

type sockaddr interface {
	sockaddr() *rawSockaddrAny
	sockport() uint
}

type sockipaddr[T any] struct {
	addr T
	port uint32
}

func (s *sockipaddr[T]) sockaddr() *rawSockaddrAny {
	ptr, plen := unsafe.Pointer(s), unsafe.Sizeof(*s)
	buf := ffiguest.BytesRead(ptr, plen)

	raddr := rawSockaddrAny{
		family: syscall.AF_INET,
	}

	switch any(s.addr).(type) {
	case sockip6:
		raddr.family = syscall.AF_INET6
	}

	copy(raddr.addr[:], buf)
	return &raddr
}

func (s *sockipaddr[T]) sockport() uint {
	return uint(s.port)
}

type sockip4 struct {
	ip [4]byte
}

type sockip6 struct {
	ip   [16]byte
	zone string
}

type sockaddrUnix struct {
	name string
}

func (s *sockaddrUnix) sockaddr() *rawSockaddrAny {
	ptr, plen := unsafe.Pointer(s), unsafe.Sizeof(*s)
	buf := ffiguest.BytesRead(ptr, plen)
	raddr := rawSockaddrAny{
		family: syscall.AF_UNIX,
	}
	copy(raddr.addr[:], buf)
	return &raddr
}

func (s *sockaddrUnix) sockport() uint {
	return 0
}

type rawSockaddrAny struct {
	family uint16
	addr   [126]byte
}

type addressBuffer struct {
	ptr unsafe.Pointer
	len uintptr
}

type iovec struct {
	ptr unsafe.Pointer
	len uintptr
}

func recvfrom(fd int, iovs [][]byte, flags int32) (n int, addr rawSockaddrAny, port, oflags int32, err error) {
	iovsBuf := make([]iovec, 0, 8)
	for _, iov := range iovs {
		iovsBuf = append(iovsBuf, iovec{
			ptr: unsafe.Pointer(unsafe.SliceData(iov)),
			len: uintptr(len(iov)),
		})
	}
	addrBuf := addressBuffer{
		ptr: unsafe.Pointer(&addr),
		len: unsafe.Sizeof(addr),
	}
	nread := int32(0)
	errno := sock_recv_from(
		int32(fd),
		unsafe.Pointer(unsafe.SliceData(iovsBuf)),
		int32(len(iovsBuf)),
		unsafe.Pointer(&addrBuf),
		flags,
		unsafe.Pointer(&port),
		unsafe.Pointer(&nread),
		unsafe.Pointer(&oflags),
	)
	if errno != 0 {
		return int(nread), addr, port, oflags, errno
	}
	runtime.KeepAlive(addrBuf)
	runtime.KeepAlive(iovsBuf)
	runtime.KeepAlive(iovs)
	return int(nread), addr, port, oflags, nil
}

func sendto(fd int, iovs [][]byte, addr rawSockaddrAny, port, flags int32) (int, error) {
	iovsBuf := make([]iovec, 0, 8)
	for _, iov := range iovs {
		iovsBuf = append(iovsBuf, iovec{
			ptr: (unsafe.Pointer(unsafe.SliceData(iov))),
			len: uintptr(len(iov)),
		})
	}
	addrBuf := addressBuffer{
		ptr: unsafe.Pointer(&addr),
		len: unsafe.Sizeof(addr),
	}
	nwritten := int32(0)
	errno := sock_send_to(
		int32(fd),
		unsafe.Pointer(unsafe.SliceData(iovsBuf)),
		int32(len(iovsBuf)),
		unsafe.Pointer(&addrBuf),
		port,
		flags,
		unsafe.Pointer(&nwritten),
	)
	if errno != 0 {
		return int(nwritten), errno
	}
	runtime.KeepAlive(addrBuf)
	runtime.KeepAlive(iovsBuf)
	runtime.KeepAlive(iovs)
	return int(nwritten), nil
}

func anyToSockaddr(rsa *rawSockaddrAny) (sockaddr, error) {
	switch rsa.family {
	case syscall.AF_INET:
		addr := (*sockipaddr[sockip4])(unsafe.Pointer(&rsa.addr))
		return addr, nil
	case syscall.AF_INET6:
		addr := (*sockipaddr[sockip6])(unsafe.Pointer(&rsa.addr))
		return addr, nil
	case syscall.AF_UNIX:
		addr := (*sockaddrUnix)(unsafe.Pointer(&rsa.addr))
		return addr, nil
	default:
		return nil, syscall.ENOTSUP
	}
}

func strlen(b []byte) (n int) {
	for n < len(b) && b[n] != 0 {
		n++
	}
	return n
}

func networkip(network string) string {
	switch network {
	case "tcp", "udp":
		return "ip"
	case "tcp4", "udp4":
		return "ip4"
	case "tcp6", "udp6":
		return "ip6"
	default:
		return ""
	}
}

func netaddr(network string, ip net.IP, port int) net.Addr {
	switch network {
	case "tcp", "tcp4", "tcp6":
		return &net.TCPAddr{IP: ip, Port: port}
	case "udp", "udp4", "udp6":
		return &net.UDPAddr{IP: ip, Port: port}
	}
	return nil
}

func newOpError(op string, addr net.Addr, err error) error {
	return &net.OpError{
		Op:   op,
		Net:  addr.Network(),
		Addr: addr,
		Err:  err,
	}
}

type netAddr struct{ network, address string }

func (na *netAddr) Network() string { return na.address }
func (na *netAddr) String() string  { return na.address }

func setNonBlock(fd int) error {
	if err := syscall.SetNonblock(fd, true); err != nil {
		return os.NewSyscallError("setnonblock", err)
	}
	return nil
}

func socketAddress(addr net.Addr) (sockaddr, error) {
	ipaddr := func(ip net.IP, zone string, port int) (sockaddr, error) {
		if ipv4 := ip.To4(); ipv4 != nil {
			return &sockipaddr[sockip4]{addr: sockip4{ip: ([4]byte)(ipv4)}, port: uint32(port)}, nil
		} else if len(ip) == net.IPv6len {
			return &sockipaddr[sockip6]{addr: sockip6{ip: ([16]byte)(ip), zone: zone}, port: uint32(port)}, nil
		} else {
			return nil, &net.AddrError{
				Err:  "unsupported address type",
				Addr: addr.String(),
			}
		}
	}

	switch a := addr.(type) {
	case *net.IPAddr:
		return ipaddr(a.IP, a.Zone, 0)
	case *net.TCPAddr:
		return ipaddr(a.IP, a.Zone, a.Port)
	case *net.UDPAddr:
		return ipaddr(a.IP, a.Zone, a.Port)
	case *net.UnixAddr:
		return &sockaddrUnix{name: a.Name}, nil
	}

	return nil, &net.AddrError{
		Err:  "unsupported address type",
		Addr: addr.String(),
	}
}

func netaddrfamily(addr net.Addr) int {
	ipfamily := func(ip net.IP) int {
		if ip.To4() == nil {
			return syscall.AF_INET6
		}

		return syscall.AF_INET
	}

	switch a := addr.(type) {
	case *net.IPAddr:
		return ipfamily(a.IP)
	case *net.TCPAddr:
		return ipfamily(a.IP)
	case *net.UDPAddr:
		return ipfamily(a.IP)
	case *net.UnixAddr:
		return syscall.AF_UNIX
	}

	return syscall.AF_INET
}

func netaddrproto(_ net.Addr) int {
	return syscall.IPPROTO_IP
}

func socketType(addr net.Addr) (int, error) {
	switch addr.Network() {
	case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
		return syscall.SOCK_STREAM, nil
	case "udp", "udp4", "udp6", "unixgram":
		return syscall.SOCK_DGRAM, nil
	default:
		return -1, syscall.EPROTOTYPE
	}
}

func setReuseAddress(fd int) error {
	if err := setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, 1); err != nil {
		// The runtime may not support the option; if that's the case and the
		// address is already in use, binding the socket will fail and we will
		// report the error then.
		switch {
		case errors.Is(err, syscall.ENOPROTOOPT):
		case errors.Is(err, syscall.EINVAL):
		default:
			return os.NewSyscallError("setsockopt", err)
		}
	}
	return nil
}

type unixConn struct {
	net.Conn
	laddr net.UnixAddr
	raddr net.UnixAddr
}

func (c *unixConn) LocalAddr() net.Addr {
	return &c.laddr
}

func (c *unixConn) RemoteAddr() net.Addr {
	return &c.raddr
}

func (c *unixConn) CloseRead() error {
	if cr, ok := c.Conn.(closeReader); ok {
		return cr.CloseRead()
	}

	return &net.OpError{
		Op:     "close",
		Net:    "unix",
		Source: c.LocalAddr(),
		Err:    syscall.ENOTSUP,
	}
}

func (c *unixConn) CloseWrite() error {
	if cw, ok := c.Conn.(closeWriter); ok {
		return cw.CloseWrite()
	}
	return &net.OpError{
		Op:     "close",
		Net:    "unix",
		Source: c.LocalAddr(),
		Err:    syscall.ENOTSUP,
	}
}

type closeReader interface {
	CloseRead() error
}

type closeWriter interface {
	CloseWrite() error
}
