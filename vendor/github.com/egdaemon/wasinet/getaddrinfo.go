package wasinet

import (
	"net"
	"strconv"
	"syscall"

	"github.com/egdaemon/wasinet/ffiguest"
)

func resolveaddrip(op, network, address string) (res []net.IP, err error) {
	if ip := net.ParseIP(address); ip != nil {
		return []net.IP{ip}, nil
	}

	if address == "" && op == oplisten {
		if networkip(network) == "ip6" {
			return []net.IP{net.IPv6zero}, nil
		}

		return []net.IP{net.IPv4zero}, nil
	}

	if address == "" {
		if networkip(network) == "ip6" {
			return []net.IP{net.IPv6loopback}, nil
		}

		return []net.IP{net.IPv4(127, 0, 0, 1)}, nil
	}

	var (
		bufreslength uint32
	)

	buf := make([]byte, 0, net.IPv6len*8)

	networkptr, networklen := ffiguest.String(network)
	addressptr, addresslen := ffiguest.String(address)
	bufptr, buflen, bufres := ffiguest.BytesResult(buf, &bufreslength)
	errno := sock_getaddrip(
		networkptr,
		networklen,
		addressptr,
		addresslen,
		bufptr,
		buflen,
		bufres,
	)

	for i := 0; i < int(bufreslength); i += net.IPv6len {
		res = append(res, net.IP(buf[i:i+net.IPv6len]))
	}

	return res, syscall.Errno(errno)
}

func resolveport(network, service string) (_port int, err error) {
	var (
		port uint32
	)

	if _port, err = strconv.Atoi(service); err == nil {
		return _port, nil
	}

	networkptr, networklen := ffiguest.String(network)
	serviceptr, servicelen := ffiguest.String(service)
	errno := sock_getaddrport(
		networkptr,
		networklen,
		serviceptr,
		servicelen,
		ffiguest.Uint32(&port),
	)

	return int(port), syscall.Errno(errno)
}
