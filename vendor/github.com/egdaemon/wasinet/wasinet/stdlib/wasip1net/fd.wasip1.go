//go:build wasip1

package wasip1net

import (
	"io"
	"net"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/egdaemon/wasinet/wasinet/ffierrors"
	"github.com/egdaemon/wasinet/wasinet/internal/errorsx"
	"github.com/egdaemon/wasinet/wasinet/stdlib/wasip1syscall"
)

const (
	readSyscallName     = "read"
	readFromSyscallName = "recvfrom"
	readMsgSyscallName  = "recvmsg"
	writeSyscallName    = "write"
	writeToSyscallName  = "sendto"
	writeMsgSyscallName = "sendmsg"
)

func loopbackIP(nnet string) net.IP {
	if nnet != "" && nnet[len(nnet)-1] == '6' {
		return net.IPv6loopback
	}
	return net.IP{127, 0, 0, 1}
}

// Network file descriptor.
type netFD struct {
	pfd pollfd

	// immutable until Close
	sysfd        int
	family       int
	sotype       int
	disconnected atomic.Bool
	net          string
	laddr        net.Addr
	raddr        net.Addr
	rsockaddr    *wasip1syscall.RawSocketAddress
}

func InitConnection(fd *netFD) (err error) {
	if err = wasip1syscall.InitializeSocketAddresses(fd.sysfd, fd.sotype, fd); err != nil {
		return err
	}

	if fd.rsockaddr, err = wasip1syscall.NetaddrToRaw(fd.family, fd.sotype, fd.raddr); err != nil {
		return err
	}

	return nil
}

func InitListener(fd *netFD) (err error) {
	if err = wasip1syscall.InitializeSocketListener(fd.sysfd, fd.sotype, fd); err != nil {
		return err
	}

	return nil
}

func newPollFD(network string, family, sotype int, sysfd int, pfd pollfd) *netFD {
	var (
		laddr net.Addr
		raddr net.Addr
	)
	// WASI preview 1 does not have functions like getsockname/getpeername,
	// so we cannot get access to the underlying IP address used by connections.
	//
	// However, listeners created by FileListener are of type *TCPListener,
	// which can be asserted by a Go program. The (*TCPListener).Addr method
	// documents that the returned value will be of type *TCPAddr, we satisfy
	// the documentpollfded behavior by creating addresses of the expected type here.
	switch network {
	case "tcp":
		laddr = new(net.TCPAddr)
		raddr = new(net.TCPAddr)
	case "udp":
		laddr = new(net.UDPAddr)
		raddr = new(net.UDPAddr)
	default:
		laddr = unknownAddr{}
		raddr = unknownAddr{}
	}

	_fd := &netFD{
		family:    family,
		sotype:    sotype,
		pfd:       pfd,
		sysfd:     sysfd,
		net:       network,
		laddr:     laddr,
		raddr:     raddr,
		rsockaddr: nil,
	}

	return _fd
}

func (fd *netFD) init(ifn func(*netFD) error) (err error) {
	defer func() {
		if err == nil {
			return
		}

		runtime.SetFinalizer(fd, (*netFD).Close)
	}()

	if err = ifn(fd); err != nil {
		return err
	}

	return fd.pfd.Init(fd.net, true)
}

func (fd *netFD) name() string {
	return "unknown"
}

func (fd *netFD) accept() (netfd *netFD, err error) {
	acceptone := func() (nfd int, err error) {
		for {
			if nfd, _, err = wasip1syscall.Accept(fd.sysfd); err == nil {
				return nfd, nil
			}

			switch ffierrors.Errno(err) {
			case syscall.EINTR, syscall.EAGAIN:
				runtime.Gosched()
			default:
				return -1, err
			}
		}
	}

	nfd, err := acceptone()
	if err != nil {
		return nil, err
	}

	netfd = newPollFD(
		fd.net,
		fd.family,
		fd.sotype,
		nfd,
		newFile(nfd, "").PollFD(),
	)

	if err = netfd.init(InitConnection); err != nil {
		netfd.Close()
		return nil, err
	}

	return netfd, nil
}

func (fd *netFD) LocalAddr() net.Addr {
	return fd.laddr
}

func (fd *netFD) RemoteAddr() net.Addr {
	return fd.raddr
}

func (fd *netFD) setAddr(laddr, raddr net.Addr) {
	fd.laddr = laddr
	fd.raddr = raddr
	runtime.SetFinalizer(fd, (*netFD).Close)
}

func (fd *netFD) Close() error {
	fd.disconnected.Store(true)
	runtime.SetFinalizer(fd, nil)
	return errorsx.Compact(
		wasip1syscall.Shutdown(fd.sysfd, syscall.SHUT_RDWR),
		fd.pfd.Close(),
	)
}

func (fd *netFD) shutdown(how int) error {
	err := errorsx.Compact(
		fd.pfd.Shutdown(how),
		wasip1syscall.Shutdown(fd.sysfd, how),
	)
	runtime.KeepAlive(fd)
	return wrapSyscallError("shutdown", err)
}

func (fd *netFD) closeRead() error {
	return fd.shutdown(syscall.SHUT_RD)
}

func (fd *netFD) closeWrite() error {
	return fd.shutdown(syscall.SHUT_WR)
}

func (fd *netFD) Read(p []byte) (n int, err error) {
	readone := func() (n int, err error) {
		for {
			if n, _, _, err = wasip1syscall.RecvFromsingle(fd.sysfd, p, nil, 0); err == nil {
				return n, zeroEOF(n)
			}

			switch ffierrors.Errno(err) {
			case syscall.EINTR, syscall.EAGAIN:
				runtime.Gosched()
			default:
				return -1, err
			}
		}
	}

	n, err = readone()
	runtime.KeepAlive(fd)
	return n, err
}

func (fd *netFD) Write(p []byte) (nn int, err error) {
	if fd.rsockaddr == nil {
		return 0, errMissingAddress
	}

	nn, err = wasip1syscall.SendToSingle(fd.sysfd, p, nil, fd.rsockaddr, 0)
	runtime.KeepAlive(fd)
	return nn, wrapSyscallError(writeSyscallName, err)
}

func (fd *netFD) SetDeadline(t time.Time) error {
	return errorsx.Compact(
		setDeadlineImpl(fd, t, wasip1syscall.SO_SNDTIMEO),
		setDeadlineImpl(fd, t, wasip1syscall.SO_RCVTIMEO),
	)
}

func (fd *netFD) SetReadDeadline(t time.Time) error {
	err := setDeadlineImpl(fd, t, wasip1syscall.SO_RCVTIMEO)
	return err
}

func (fd *netFD) SetWriteDeadline(t time.Time) error {
	err := setDeadlineImpl(fd, t, wasip1syscall.SO_SNDTIMEO)
	return err
}

func zeroEOF(n int) error {
	if n == 0 {
		return io.EOF
	}

	return nil
}
func (fd *netFD) readMsg(b, oob []byte) (n, oobn int, flags int, rsa wasip1syscall.RawSocketAddress, err error) {
	readone := func() (n int, flags int, rsa wasip1syscall.RawSocketAddress, err error) {
		for {
			var rflags int32
			if n, rsa, rflags, err = wasip1syscall.RecvFromsingle(fd.sysfd, b, oob, 0); err == nil {
				return n, int(rflags), rsa, zeroEOF(n)
			}

			switch ffierrors.Errno(err) {
			case syscall.EINTR, syscall.EAGAIN:
				runtime.Gosched()
			default:
				return n, int(rflags), rsa, err
			}
		}
	}

	n, flags, rsa, err = readone()
	return n, oobn, flags, rsa, err
}

func (fd *netFD) writeTo(p []byte, sa *wasip1syscall.RawSocketAddress) (n int, err error) {
	n, err = wasip1syscall.SendToSingle(fd.sysfd, p, nil, sa, 0)
	runtime.KeepAlive(fd)
	return n, wrapSyscallError(writeToSyscallName, err)
}

func (fd *netFD) writeMsg(p []byte, oob []byte, sa *wasip1syscall.RawSocketAddress) (n int, oobn int, err error) {
	n, err = wasip1syscall.SendToSingle(fd.sysfd, p, oob, sa, 0)
	runtime.KeepAlive(fd)
	return n, oobn, wrapSyscallError(writeMsgSyscallName, err)
}

func (fd *netFD) dup() (f *os.File, err error) {
	ns, call, err := fd.pfd.Dup()
	if err != nil {
		if call != "" {
			err = os.NewSyscallError(call, err)
		}
		return nil, err
	}

	return newFile(ns, fd.name()), nil
}

// wasip1syscall.SO_RCVTIMEO
func setDeadlineImpl(fd *netFD, t time.Time, mode uint32) error {
	var d time.Duration
	if !t.IsZero() {
		d = time.Until(t)
		if d == 0 {
			d = -1 // don't confuse deadline right now with no deadline
		}
	}

	return wasip1syscall.SetsockoptTimeval(fd.sysfd, uint32(fd.sotype), mode, d)
}
