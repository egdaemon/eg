package wasip1net

import (
	"net"
	"os"
)

func Socket(fd uintptr) *os.File {
	return newFile(int(fd), "")
}

func Conn(family, sotype int, f *os.File) (c net.Conn, err error) {
	return newFileConn(family, sotype, f)
}

func Listen(family, sotype int, fd uintptr) (net.Listener, error) {
	return Listener(family, sotype, fd)
}
