package netx

import (
	"fmt"
	"log"
	"net"
	"syscall"

	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/errorsx"
)

// NewNoopListener -- junk listener.
func NewNoopListener() net.Listener {
	return noopListener{}
}

type noopListener struct{}

func (noopListener) Accept() (net.Conn, error) {
	return nil, fmt.Errorf("noopListener can not accept connections")
}

func (noopListener) Close() error {
	return nil
}

func (noopListener) Addr() net.Addr {
	return &net.UnixAddr{
		Name: "foo",
		Net:  "unix",
	}
}

func AddrToString(addrs ...*net.TCPAddr) []string {
	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, addr.String())
	}

	return result
}

// intercept read and write calls
func DebugConn(format string, c net.Conn) net.Conn {
	if pc, ok := c.(net.PacketConn); ok {
		return debugpacketconn{format: format, PacketConn: pc}
	}
	return debugconn{format: format, Conn: c}
}

type debugconn struct {
	format string
	net.Conn
}

func (t debugconn) Read(b []byte) (n int, err error) {
	log.Println("net.Conn.Read initiated", t.Conn.RemoteAddr().String(), fmt.Sprintf(t.format, bytesx.Debug(b)))
	defer func() {
		log.Println("net.Conn.Read completed", t.Conn.RemoteAddr().String(), fmt.Sprintf(t.format, bytesx.Debug(b)))
	}()

	return t.Conn.Read(b)
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (t debugconn) Write(b []byte) (n int, err error) {
	log.Println("net.Conn.Write initiated", t.Conn.RemoteAddr().String(), fmt.Sprintf(t.format, bytesx.Debug(b)))
	defer func() {
		log.Println("net.Conn.Write completed", t.Conn.RemoteAddr().String(), fmt.Sprintf(t.format, bytesx.Debug(b)))
	}()

	return t.Conn.Write(b)
}

type debugpacketconn struct {
	format string
	net.PacketConn
}

func (t debugpacketconn) RemoteAddr() net.Addr {
	c, ok := t.PacketConn.(net.Conn)
	if !ok {
		return nil
	}

	return c.RemoteAddr()
}
func (t debugpacketconn) Read(b []byte) (n int, err error) {
	c, ok := t.PacketConn.(net.Conn)
	if !ok {
		return 0, syscall.ENOTSUP
	}

	log.Println("net.Conn.Read initiated", c.RemoteAddr().String(), fmt.Sprintf(t.format, bytesx.Debug(b)))
	defer func() {
		log.Println("net.Conn.Read completed", c.RemoteAddr().String(), fmt.Sprintf(t.format, bytesx.Debug(b)), fmt.Sprintf(t.format, errorsx.Stack()))
	}()

	return c.Read(b)
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (t debugpacketconn) Write(b []byte) (n int, err error) {
	c, ok := t.PacketConn.(net.Conn)
	if !ok {
		return 0, syscall.ENOTSUP
	}
	log.Println("net.Conn.Write initiated", c.RemoteAddr().String(), fmt.Sprintf(t.format, bytesx.Debug(b)))
	defer func() {
		log.Println("net.Conn.Write completed", c.RemoteAddr().String(), fmt.Sprintf(t.format, bytesx.Debug(b)), fmt.Sprintf(t.format, errorsx.Stack()))
	}()

	return c.Write(b)
}

func (t debugpacketconn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	log.Println("net.PacketConn.ReadFrom initiated", fmt.Sprintf(t.format, bytesx.Debug(b)))
	defer func() {
		result := "completed"
		if err != nil {
			result = "failed"
		}
		log.Println("net.PacketConn.ReadFrom", result, addr, n, fmt.Sprintf(t.format, bytesx.Debug(b)))
	}()
	return t.PacketConn.ReadFrom(b)
}

func (t debugpacketconn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	log.Println("net.PacketConn.WriteTo initiated", addr, fmt.Sprintf(t.format, bytesx.Debug(b)))
	defer func() {
		result := "completed"
		if err != nil {
			result = "failed"
		}
		log.Println("net.PacketConn.WriteTo", result, addr, n, fmt.Sprintf(t.format, bytesx.Debug(b)))
	}()
	return t.PacketConn.WriteTo(b, addr)
}
