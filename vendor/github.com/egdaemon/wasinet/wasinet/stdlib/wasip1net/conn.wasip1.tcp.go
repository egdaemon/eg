//go:build wasip1

package wasip1net

// TCPConn is an implementation of the [Conn] interface for TCP network
// connections.
type TCPConn struct {
	conn
}
