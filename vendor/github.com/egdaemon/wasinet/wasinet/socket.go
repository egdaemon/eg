package wasinet

import (
	"net"
	"syscall"
)

const (
	oplisten = "listen"
	opdial   = "dial"
)

func netaddrproto(addr net.Addr) int {
	switch addr.Network() {
	case "tcp6", "udp6":
		return syscall.IPPROTO_IPV6
	case "tcp4", "udp4":
		return syscall.IPPROTO_IP
	case "unix", "unixpacket":
		return syscall.IPPROTO_IP
	default:
		return syscall.IPPROTO_IP
	}
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
