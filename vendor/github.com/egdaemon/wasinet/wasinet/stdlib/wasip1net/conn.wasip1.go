//go:build wasip1

package wasip1net

import (
	"io"
	"net"
	"os"
	"syscall"
	"time"
)

type conn struct {
	fd *netFD
}

func (c *conn) ok() bool { return c != nil && c.fd != nil }

// Implementation of the Conn interface.

// Read implements the Conn Read method.
func (c *conn) Read(b []byte) (int, error) {
	if !c.ok() {
		return 0, syscall.EINVAL
	}

	n, err := c.fd.Read(b)
	if err != nil && err != io.EOF {
		err = &net.OpError{Op: "read", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return n, err
}

// Write implements the Conn Write method.
func (c *conn) Write(b []byte) (int, error) {
	if !c.ok() {
		return 0, syscall.EINVAL
	}
	n, err := c.fd.Write(b)
	if err != nil {
		err = &net.OpError{Op: "write", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return n, err
}

// Close closes the connection.
func (c *conn) Close() error {
	defer func() {
		c.fd = nil
	}()
	if !c.ok() {
		return syscall.EINVAL
	}
	err := c.fd.Close()
	if err != nil {
		err = &net.OpError{Op: "close", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return err
}

// LocalAddr returns the local network address.
// The Addr returned is shared by all invocations of LocalAddr, so
// do not modify it.
func (c *conn) LocalAddr() net.Addr {
	if !c.ok() {
		return nil
	}
	return c.fd.laddr
}

// RemoteAddr returns the remote network address.
// The Addr returned is shared by all invocations of RemoteAddr, so
// do not modify it.
func (c *conn) RemoteAddr() net.Addr {
	if !c.ok() {
		return nil
	}
	return c.fd.raddr
}

// SetDeadline implements the Conn SetDeadline method.
func (c *conn) SetDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := c.fd.SetDeadline(t); err != nil {
		return &net.OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// SetReadDeadline implements the Conn SetReadDeadline method.
func (c *conn) SetReadDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := c.fd.SetReadDeadline(t); err != nil {
		return &net.OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// SetWriteDeadline implements the Conn SetWriteDeadline method.
func (c *conn) SetWriteDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := c.fd.SetWriteDeadline(t); err != nil {
		return &net.OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// SetReadBuffer sets the size of the operating system's
// receive buffer associated with the connection.
func (c *conn) SetReadBuffer(bytes int) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := setReadBuffer(c.fd, bytes); err != nil {
		return &net.OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// SetWriteBuffer sets the size of the operating system's
// transmit buffer associated with the connection.
func (c *conn) SetWriteBuffer(bytes int) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := setWriteBuffer(c.fd, bytes); err != nil {
		return &net.OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// File returns a copy of the underlying [os.File].
// It is the caller's responsibility to close f when finished.
// Closing c does not affect f, and closing f does not affect c.
//
// The returned os.File's file descriptor is different from the connection's.
// Attempting to change properties of the original using this duplicate
// may or may not have the desired effect.
func (c *conn) File() (f *os.File, err error) {
	f, err = c.fd.dup()
	if err != nil {
		err = &net.OpError{Op: "file", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return
}

func (c *conn) SyscallConn() syscall.RawConn {
	return netsysconn{*c}
}

type netsysconn struct {
	conn
}

func (t netsysconn) Control(f func(fd uintptr)) error {
	return t.fd.pfd.RawControl(f)
}

func (t netsysconn) Read(f func(fd uintptr) (done bool)) error {
	return t.fd.pfd.RawRead(f)
}

// Write is like Read but for writing.
func (t netsysconn) Write(f func(fd uintptr) (done bool)) error {
	return t.fd.pfd.RawWrite(f)
}
