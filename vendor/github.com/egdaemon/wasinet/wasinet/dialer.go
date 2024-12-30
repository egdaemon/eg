package wasinet

import (
	"context"
	"log/slog"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/egdaemon/wasinet/wasinet/ffierrors"
	"github.com/egdaemon/wasinet/wasinet/internal/errorsx"
	"github.com/egdaemon/wasinet/wasinet/stdlib/wasip1net"
	"github.com/egdaemon/wasinet/wasinet/stdlib/wasip1syscall"
)

// Dialer is a type similar to net.Dialer but it uses the dial functions defined
// in this package instead of those from the standard library.
//
// For details about the configuration, see: https://pkg.go.dev/net#Dialer
//
// Note that depending on the WebAssembly runtime being employed, certain
// functionalities of the Dialer may not be available.
type Dialer struct {
	Timeout        time.Duration
	Deadline       time.Time
	LocalAddr      net.Addr
	DualStack      bool
	FallbackDelay  time.Duration
	Resolver       *net.Resolver   // ignored
	Cancel         <-chan struct{} // ignored
	Control        func(network, address string, c syscall.RawConn) error
	ControlContext func(ctx context.Context, network, address string, c syscall.RawConn) error
}

func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	timeout := d.Timeout
	if !d.Deadline.IsZero() {
		dl := max(0, time.Until(d.Deadline))
		timeout = min(max(d.Timeout, dl), dl)
	}

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if d.LocalAddr != nil {
		slog.WarnContext(ctx, "wasip1.Dialer: LocalAddr not yet supported on GOOS=wasip1")
	}
	if d.Resolver != nil {
		slog.WarnContext(ctx, "wasip1.Dialer: Resolver ignored because it is not supported on GOOS=wasip1")
	}
	if d.Cancel != nil {
		slog.WarnContext(ctx, "wasip1.Dialer: Cancel channel not implemented on GOOS=wasip1")
	}
	if d.Control != nil {
		slog.WarnContext(ctx, "wasip1.Dialer: Control function not yet supported on GOOS=wasip1")
	}
	if d.ControlContext != nil {
		slog.WarnContext(ctx, "wasip1.Dialer: ControlContext function not yet supported on GOOS=wasip1")
	}
	// TOOD:
	// - use LocalAddr to bind to a socket prior to establishing the connection
	// - use DualStack and FallbackDelay
	// - use Control and ControlContext functions
	// - emulate the Cancel channel with context.Context
	return DialContext(ctx, network, address)
}

// Dial connects to the address on the named network.
func Dial(network, address string) (net.Conn, error) {
	return DialContext(context.Background(), network, address)
}

// DialContext is a variant of Dial that accepts a context.
func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	addrs, err := wasip1syscall.LookupAddress(ctx, opdial, network, address)
	if err != nil {
		return nil, netOpErr(opdial, unresolvedaddr(network, address), err)
	}

	for _, addr := range addrs {
		var conn net.Conn
		conn, err = dialAddr(ctx, addr)
		if err == nil {
			return conn, nil
		}

		if ctx.Err() != nil {
			break
		}
	}

	return nil, netOpErr(opdial, unresolvedaddr(network, address), err)
}

func dialAddr(ctx context.Context, addr net.Addr) (_ net.Conn, err error) {
	defer func() {
		if err == nil {
			return
		}
	}()

	af := wasip1syscall.NetaddrAFFamily(addr)
	sotype, err := socketType(addr)
	if err != nil {
		return nil, err
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

	if sotype == syscall.SOCK_DGRAM && af != syscall.AF_UNIX {
		if err := wasip1syscall.SetSockoptBroadcast(fd); err != nil {
			return nil, err
		}
	}

	caddr, err := wasip1syscall.NetaddrToRaw(af, sotype, addr)
	if err != nil {
		return nil, err
	}

	var inProgress bool
	if err := wasip1syscall.Connect(fd, caddr); err != nil {
		switch errno := ffierrors.Errno(err); errno {
		case syscall.EINPROGRESS:
			inProgress = true
		default:
			return nil, errno
		}
	}

	if sotype == syscall.SOCK_DGRAM {
		defer func() {
			fd = -1 // now the *netFD owns the file descriptor
		}()

		return wasip1net.Conn(af, sotype, wasip1net.Socket(uintptr(fd)))
	}

	sconn := wasip1net.Socket(uintptr(fd))
	fd = -1 // now the *netFD owns the file descriptor
	defer func() {
		if err == nil {
			return
		}
		_ = sconn.Close()
	}()
	if inProgress {
		rawConn, err := sconn.SyscallConn()
		if err != nil {
			return nil, err
		}
		errch := make(chan error)
		go func() {
			var err error
			cerr := rawConn.Write(func(fd uintptr) bool {
				var value int
				value, err = wasip1syscall.GetSockoptInt(int(fd), SOL_SOCKET, wasip1syscall.SO_ERROR)
				if err != nil {
					return true // done
				}

				switch ffierrors.Errno(err) {
				case syscall.EINPROGRESS, syscall.EINTR:
					return false // continue
				case syscall.EISCONN:
					err = nil
					return true
				case ffierrors.ErrnoSuccess():
					// The net poller can wake up spuriously. Check that we are
					// are really connected.
					_, err := wasip1syscall.Getpeername(int(fd))
					return err == nil
				default:
					err = syscall.Errno(value)
					return true
				}
			})
			errch <- errorsx.Compact(err, cerr)
		}()

		select {
		case err := <-errch:
			if err != nil {
				sconn.Close()
				return nil, os.NewSyscallError("connect", err)
			}
		case <-ctx.Done():
			// This should interrupt the async connect operation handled by the
			// goroutine.
			sconn.Close()
			// Wait for the goroutine to complete, we can safely discard the
			// error here because we don't care about the socket anymore.
			<-errch

			return nil, context.Cause(ctx)
		}
	}

	return wasip1net.Conn(af, sotype, sconn)
}
