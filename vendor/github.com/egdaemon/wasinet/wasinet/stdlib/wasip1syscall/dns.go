package wasip1syscall

import (
	"context"
	"net"
	"os"
	"runtime"
	"strconv"
	"syscall"

	"github.com/egdaemon/wasinet/wasinet/ffi"
	"github.com/egdaemon/wasinet/wasinet/ffierrors"
)

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

func LookupAddress(_ context.Context, op, network, address string) ([]net.Addr, error) {
	switch network {
	case "unix", "unixgram":
		return []net.Addr{&net.UnixAddr{Name: address, Net: network}}, nil
	default:
	}

	hostname, service, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	port, err := ResolvePort(network, service)
	if err != nil {
		return nil, os.NewSyscallError("resolveport", err)
	}

	ips, err := ResolveAddrip(op, network, hostname)
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

func ResolveAddrip(op, network, address string) (res []net.IP, err error) {
	if ip := net.ParseIP(address); ip != nil {
		return []net.IP{ip}, nil
	}

	netip := networkip(network)

	if address == "" && op == oplisten {
		if netip == "ip6" {
			return []net.IP{net.IPv6zero}, nil
		}

		return []net.IP{net.IPv4zero}, nil
	}

	if address == "" {
		if netip == "ip6" {
			return []net.IP{net.IPv6loopback}, nil
		}

		return []net.IP{net.IPv4(127, 0, 0, 1)}, nil
	}

	var (
		bufreslength uint32
	)

	buf := make([]byte, net.IPv6len*8)

	networkptr, networklen := ffi.String(netip)
	addressptr, addresslen := ffi.String(address)
	bufptr, buflen := ffi.Slice(buf)
	bufresptr, _ := ffi.Pointer(&bufreslength)

	errno := sock_getaddrip(
		networkptr,
		networklen,
		addressptr,
		addresslen,
		bufptr,
		buflen,
		bufresptr,
	)
	runtime.KeepAlive(netip)
	runtime.KeepAlive(address)
	runtime.KeepAlive(buf)

	if err = ffierrors.Error(errno); err != nil {
		return nil, err
	}

	for i := 0; i < int(bufreslength); i += net.IPv6len {
		res = append(res, net.IP(buf[i:i+net.IPv6len]))
	}

	return res, nil
}

func ResolvePort(network, service string) (_port int, err error) {
	var (
		port uint32
	)

	if _port, err = strconv.Atoi(service); err == nil {
		return _port, nil
	}

	networkptr, networklen := ffi.String(network)
	serviceptr, servicelen := ffi.String(service)
	portptr, _ := ffi.Pointer(&port)
	errno := sock_getaddrport(
		networkptr,
		networklen,
		serviceptr,
		servicelen,
		portptr,
	)

	return int(port), syscall.Errno(errno)
}
