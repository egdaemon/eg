package wasinet

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/egdaemon/wasinet/ffi"
	"github.com/egdaemon/wasinet/internal/errorsx"
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
	rsa := sa.sockaddr()
	rawaddr, rawaddrlen := ffi.Pointer(&rsa)
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
	rsa := sa.sockaddr()
	rawaddr, rawaddrlen := ffi.Pointer(&rsa)
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
	var rsa rawsocketaddr
	rsaptr, rsalength := ffi.Pointer(&rsa)
	errno := sock_getlocaladdr(int32(fd), rsaptr, rsalength)
	if errno != 0 {
		return nil, errno
	}

	return rawtosockaddr(&rsa)
}

func getpeername(fd int) (sockaddr, error) {
	var rsa rawsocketaddr

	rsaptr, rsalength := ffi.Pointer(&rsa)
	errno := sock_getpeeraddr(int32(fd), rsaptr, rsalength)
	if errno != 0 {
		return nil, errno
	}

	return rawtosockaddr(&rsa)
}

type sockaddr interface {
	sockaddr() rawsocketaddr
}

type sockipaddr[T any] struct {
	addr T
	port uint32
}

func (s sockipaddr[T]) sockaddr() rawsocketaddr {
	ptr, plen := ffi.Pointer(&s)
	buf := errorsx.Must(ffi.ReadSlice[byte](ffi.Native{}, ptr, plen))
	raddr := rawsocketaddr{}

	switch x := any(s.addr).(type) {
	case sockip4:
		raddr.family = uint16(sock_determine_host_af_family(syscall.AF_INET))
	case sockip6:
		raddr.family = uint16(sock_determine_host_af_family(syscall.AF_INET6))
	default:
		log.Printf("unknown socket type %T - default famimly to AF_INET\n", x)
		raddr.family = uint16(sock_determine_host_af_family(syscall.AF_INET))
	}

	copy(raddr.addr[:], buf)
	return raddr
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

func (s *sockaddrUnix) sockaddr() rawsocketaddr {
	ptr, plen := ffi.Pointer(&s)
	buf := errorsx.Must(ffi.ReadSlice[byte](ffi.Native{}, ptr, plen))

	raddr := rawsocketaddr{
		family: uint16(sock_determine_host_af_family(syscall.AF_UNIX)),
	}
	copy(raddr.addr[:], buf)
	return raddr
}

type rawsocketaddr struct {
	family uint16
	addr   [126]byte
}

func recvfrom(fd int, iovs [][]byte, flags int32) (n int, addr rawsocketaddr, oflags int32, err error) {
	iovsptr, iovslen := ffi.Slice(iovs)
	addrptr, _ := ffi.Pointer(&addr)
	errno := sock_recv_from(
		int32(fd),
		iovsptr, iovslen,
		addrptr,
		flags,
		unsafe.Pointer(&n),
		unsafe.Pointer(&oflags),
	)
	runtime.KeepAlive(addrptr)
	runtime.KeepAlive(iovsptr)
	runtime.KeepAlive(iovs)
	return n, addr, oflags, ffi.ErrErrno(errno)
}

func sendto(fd int, iovs [][]byte, addr rawsocketaddr, flags int32) (int, error) {
	vecs := ffi.SliceVector(iovs...)
	iovsptr, iovslen := ffi.Slice(vecs)
	addrptr, addrlen := ffi.Pointer(&addr)

	nwritten := int(0)
	errno := sock_send_to(
		int32(fd),
		iovsptr, iovslen,
		addrptr, addrlen,
		flags,
		unsafe.Pointer(&nwritten),
	)
	runtime.KeepAlive(addr)
	runtime.KeepAlive(iovs)
	return nwritten, ffi.ErrErrno(errno)
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
			return sockipaddr[sockip4]{addr: sockip4{ip: ([4]byte)(ipv4)}, port: uint32(port)}, nil
		} else if len(ip) == net.IPv6len {
			return sockipaddr[sockip6]{addr: sockip6{ip: ([16]byte)(ip), zone: zone}, port: uint32(port)}, nil
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
