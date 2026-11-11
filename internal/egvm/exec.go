package egvm

import (
	"encoding/binary"
	"io"
)

// Port is the vsock port vminit serves commands on.
const Port = 1024

// NFSPort is the vsock port the host serves the machine's root on. vminit forwards the guest's NFS client to it from
// the same port on the guest's loopback.
const NFSPort = 2049

// The streams of a command, each its own connection to vminit. A connection starts with a header naming the command
// and the stream it carries. The control stream carries the command's execproxy.ExecRequest from the host and its
// execproxy.ExecResponse back, both length delimited; the host closing it before the response kills the command.
const (
	StreamStdin uint8 = iota
	StreamStdout
	StreamStderr
	StreamControl
)

// Header begins every connection to vminit.
type Header struct {
	ID     uint64
	Stream uint8
}

// WriteHeader writes the header of the stream of command id to w.
func WriteHeader(w io.Writer, id uint64, stream uint8) error {
	return binary.Write(w, binary.BigEndian, Header{ID: id, Stream: stream})
}

// ReadHeader reads the header a connection starts with.
func ReadHeader(r io.Reader) (h Header, err error) {
	return h, binary.Read(r, binary.BigEndian, &h)
}
