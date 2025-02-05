//go:build wasip1

package wasip1net

// UnixConn is an implementation of the [Conn] interface for unix network
// connections.
type UnixConn struct {
	conn
}
