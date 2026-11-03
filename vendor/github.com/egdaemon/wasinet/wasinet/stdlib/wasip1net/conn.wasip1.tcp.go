//go:build wasip1

package wasip1net

import "syscall"

// TCPConn is an implementation of the [Conn] interface for TCP network
// connections.
type TCPConn struct {
	*conn
}

// CloseRead shuts down the reading side of the TCP connection.
func (c *TCPConn) CloseRead() error {
	return c.shutdown(syscall.SHUT_RD)
}

// CloseWrite shuts down the writing side of the TCP connection.
func (c *TCPConn) CloseWrite() error {
	return c.shutdown(syscall.SHUT_WR)
}
