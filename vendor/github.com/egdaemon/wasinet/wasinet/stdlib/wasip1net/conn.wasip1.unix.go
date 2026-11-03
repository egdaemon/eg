//go:build wasip1

package wasip1net

import "syscall"

// UnixConn is an implementation of the [Conn] interface for unix network
// connections.
type UnixConn struct {
	*conn
}

// CloseRead shuts down the reading side of the Unix connection.
func (c *UnixConn) CloseRead() error {
	return c.shutdown(syscall.SHUT_RD)
}

// CloseWrite shuts down the writing side of the Unix connection.
func (c *UnixConn) CloseWrite() error {
	return c.shutdown(syscall.SHUT_WR)
}
