//go:build !wasip1

package wasip1net

import (
	"net"
	"os"
)

func newFile(fd int, name string) *os.File {
	return os.NewFile(uintptr(fd), name)
}

func newFileConn(family, sotype int, f *os.File) (c net.Conn, err error) {
	return net.FileConn(f)
}

func PacketConnFd(family, sotype int, fd uintptr) (*pconn, error) {
	pc, err := net.FilePacketConn(Socket(uintptr(fd)))
	if err != nil {
		return nil, err
	}
	return makePacketConn(pc), nil
}

func Listener(family, sotype int, fd uintptr) (net.Listener, error) {
	return net.FileListener(Socket(fd))
}
