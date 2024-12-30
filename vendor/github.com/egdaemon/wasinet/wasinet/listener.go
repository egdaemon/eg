package wasinet

import (
	"context"
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/egdaemon/wasinet/wasinet/stdlib/wasip1net"
	"github.com/egdaemon/wasinet/wasinet/stdlib/wasip1syscall"
)

// Listen announces on the local network address.
func Listen(ctx context.Context, network, address string) (net.Listener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6", "unix":
	default:
		return nil, unsupportedNetwork(network, address)
	}

	addrs, err := wasip1syscall.LookupAddress(ctx, oplisten, network, address)
	if err != nil {
		return nil, netOpErr(oplisten, unresolvedaddr(network, address), err)
	}

	firstaddr := addrs[0]
	lstn, err := listenAddr(firstaddr)
	return lstn, netOpErr(oplisten, firstaddr, err)
}

// ListenPacket creates a listening packet connection.
func ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	switch network {
	case "udp", "udp4", "udp6", "unixgram":
	default:
		return nil, unsupportedNetwork(network, address)
	}
	addrs, err := wasip1syscall.LookupAddress(ctx, oplisten, network, address)
	if err != nil {
		return nil, netOpErr(oplisten, unresolvedaddr(network, address), err)
	}

	conn, err := listenPacketAddr(addrs[0])
	return conn, netOpErr(oplisten, addrs[0], err)
}

func unsupportedNetwork(network, address string) error {
	return fmt.Errorf("unsupported network: %s://%s", network, address)
}

func listenAddr(addr net.Addr) (net.Listener, error) {
	af := wasip1syscall.NetaddrAFFamily(addr)
	sotype, err := socketType(addr)
	if err != nil {
		return nil, os.NewSyscallError("socket", err)
	}
	fd, err := wasip1syscall.Socket(af, sotype, netaddrproto(addr))
	if err != nil {
		return nil, err
	}
	defer func() {
		if fd >= 0 {
			syscall.Close(fd)
		}
	}()

	if err := wasip1syscall.SetReuseAddress(fd); err != nil {
		return nil, err
	}

	baddr, err := wasip1syscall.NetaddrToRaw(af, sotype, addr)
	if err != nil {
		return nil, os.NewSyscallError("raw address", err)
	}

	if err := wasip1syscall.Bind(fd, baddr); err != nil {
		return nil, err
	}

	const backlog = 64 // TODO: configurable?
	if err := wasip1syscall.Listen(fd, backlog); err != nil {
		return nil, err
	}

	l, err := wasip1net.Listen(af, sotype, uintptr(fd))
	if err != nil {
		return nil, err
	}
	fd = -1 // now the *os.File owns the file descriptor

	return l, nil
}

func listenPacketAddr(addr net.Addr) (net.PacketConn, error) {
	af := wasip1syscall.NetaddrAFFamily(addr)
	sotype, err := socketType(addr)
	if err != nil {
		return nil, os.NewSyscallError("socket", err)
	}
	fd, err := wasip1syscall.Socket(af, sotype, netaddrproto(addr))
	if err != nil {
		return nil, os.NewSyscallError("socket", err)
	}
	defer func() {
		if fd >= 0 {
			syscall.Close(fd)
		}
	}()

	if err := wasip1syscall.SetReuseAddress(fd); err != nil {
		return nil, os.NewSyscallError("set_socketopt_int", err)
	}

	baddr, err := wasip1syscall.NetaddrToRaw(af, sotype, addr)
	if err != nil {
		return nil, os.NewSyscallError("bind", err)
	}

	if err := wasip1syscall.Bind(fd, baddr); err != nil {
		return nil, os.NewSyscallError("bind", err)
	}

	pconn, err := wasip1net.PacketConnFd(af, sotype, uintptr(fd))
	if err != nil {
		return nil, err
	}
	fd = -1 // now the *netFD owns the file descriptor
	return pconn, err
}
