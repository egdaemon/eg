package wasip1syscall

import (
	"log"
	"net"
	"syscall"

	"github.com/egdaemon/wasinet/wasinet/internal/langx"
)

type addressable interface {
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

func InitializeSocketAddresses(fd int, sotype int, dst addressable) (err error) {
	switch addrdst := dst.LocalAddr().(type) {
	default:
		var (
			v sockaddr
		)

		if v, err = Getsockname(int(fd)); err != nil {
			log.Printf("sockname %T - %v\n", addrdst, err)
			return err
		}

		setnetaddr(sotype, addrdst, v)
	}

	switch addrdst := dst.RemoteAddr().(type) {
	default:
		var (
			v sockaddr
		)

		if v, err = Getpeername(int(fd)); err != nil {
			log.Printf("peername %T - %s - %v\n", addrdst, addrdst.String(), err)
			return err
		}

		setnetaddr(sotype, addrdst, v)
	}

	return nil
}

func InitializeSocketListener(fd int, sotype int, dst addressable) (err error) {
	switch addrdst := dst.LocalAddr().(type) {
	default:
		var (
			v sockaddr
		)

		if v, err = Getsockname(int(fd)); err != nil {
			log.Printf("sockname %T - %v\n", addrdst, err)
			return err
		}

		setnetaddr(sotype, addrdst, v)
	}

	return nil
}

func setnetaddr(sotype int, dst net.Addr, src sockaddr) {
	if src == nil {
		return
	}

	switch a := dst.(type) {
	case *net.IPAddr:
		*a = langx.DerefOrZero(ipNetAddr(src))
	case *net.TCPAddr:
		*a = langx.DerefOrZero(tcpNetAddr(src))
	case *net.UDPAddr:
		*a = langx.DerefOrZero(udpNetAddr(src))
	case *net.UnixAddr:
		switch sotype {
		case syscall.SOCK_DGRAM:
			*a = langx.DerefOrZero(unixgramNetAddr(src))
		default:
			*a = langx.DerefOrZero(unixNetAddr(src))
		}
	default:
		log.Printf("unable to set addr: %T\n", dst)
	}
}

func SocketAddressFormat(family, sotype int) func(sa sockaddr) net.Addr {
	switch int32(family) {
	case AF().INET, AF().INET6:
		switch sotype {
		case syscall.SOCK_STREAM:
			return func(sa sockaddr) net.Addr { return tcpNetAddr(sa) }
		case syscall.SOCK_DGRAM:
			return func(sa sockaddr) net.Addr { return udpNetAddr(sa) }
		case syscall.SOCK_RAW:
			return func(sa sockaddr) net.Addr { return ipNetAddr(sa) }
		}
	case AF().UNIX:
		switch sotype {
		case syscall.SOCK_STREAM:
			return func(sa sockaddr) net.Addr { return unixNetAddr(sa) }
		case syscall.SOCK_DGRAM:
			return func(sa sockaddr) net.Addr { return unixgramNetAddr(sa) }
		case syscall.SOCK_SEQPACKET:
			return func(sa sockaddr) net.Addr { return unixpacketNetAddr(sa) }
		}
	}

	log.Println(family, AF().INET, AF().INET6, AF().UNIX, "|", sotype, syscall.SOCK_STREAM, syscall.SOCK_DGRAM)
	return func(sa sockaddr) net.Addr { return nil }
}

func unixNetAddr(sa sockaddr) *net.UnixAddr {
	if sa == nil {
		return &net.UnixAddr{}
	}
	switch proto := sa.(type) {
	case *addressany[addrunix]:
		return &net.UnixAddr{Name: proto.addr.Path(), Net: "unix"}
	default:
		return nil
	}
}

func unixgramNetAddr(sa sockaddr) *net.UnixAddr {
	if sa == nil {
		return &net.UnixAddr{}
	}
	switch proto := sa.(type) {
	case *addressany[addrunix]:
		return &net.UnixAddr{Name: proto.addr.Path(), Net: "unixgram"}
	default:
		return nil
	}
}

func unixpacketNetAddr(sa sockaddr) *net.UnixAddr {
	if sa == nil {
		return &net.UnixAddr{}
	}
	switch proto := sa.(type) {
	case *addressany[addrunix]:
		return &net.UnixAddr{Name: proto.addr.Path(), Net: "unixpacket"}
	}
	return nil
}

func tcpNetAddr(sa sockaddr) *net.TCPAddr {
	if sa == nil {
		return &net.TCPAddr{}
	}

	switch unknown := sa.(type) {
	case *addressany[addrip4]:
		return &net.TCPAddr{IP: unknown.addr.ip[:], Port: int(unknown.addr.port)}
	case *addressany[addrip6]:
		return &net.TCPAddr{IP: unknown.addr.ip[:], Port: int(unknown.addr.port), Zone: ""}
	}
	return nil
}

func udpNetAddr(sa sockaddr) *net.UDPAddr {
	if sa == nil {
		return &net.UDPAddr{}
	}

	switch unknown := sa.(type) {
	case *addressany[addrip4]:
		return &net.UDPAddr{IP: unknown.addr.ip[:], Port: int(unknown.addr.port)}
	case *addressany[addrip6]:
		return &net.UDPAddr{IP: unknown.addr.ip[:], Port: int(unknown.addr.port), Zone: ""}
	default:
		return nil
	}
}

func ipNetAddr(sa sockaddr) *net.IPAddr {
	if sa == nil {
		return &net.IPAddr{}
	}

	switch proto := sa.(type) {
	case *addressany[addrip4]:
		return &net.IPAddr{IP: proto.addr.ip[0:]}
	case *addressany[addrip6]:
		return &net.IPAddr{IP: proto.addr.ip[0:]}
	default:
		return nil
	}
}
