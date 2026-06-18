package duckproxy

import (
	"encoding/binary"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

// maxFrameLen bounds the length prefix read off the wire -- a sanity cap
// against a corrupt or hostile peer, not a real limit on query/result size.
const maxFrameLen = 64 << 20 // 64MiB

// WriteFrame writes msg to w as a 4-byte big-endian length prefix followed
// by its marshaled bytes. This -- not gRPC, not gob -- is the entire
// transport layer: a single connection carries one ClientFrame/ServerFrame
// at a time, synchronously, since database/sql never calls a driver.Conn
// concurrently from multiple goroutines.
func WriteFrame(w io.Writer, msg proto.Message) error {
	b, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	buf := make([]byte, 4+len(b))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(b)))
	copy(buf[4:], b)

	_, err = w.Write(buf)
	return err
}

// ReadFrame reads one length-prefixed frame from r and unmarshals it into
// msg.
func ReadFrame(r io.Reader, msg proto.Message) error {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return err
	}

	n := binary.BigEndian.Uint32(lenBuf[:])
	if n > maxFrameLen {
		return fmt.Errorf("duckproxy: frame of %d bytes exceeds %d byte limit", n, maxFrameLen)
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}

	return proto.Unmarshal(buf, msg)
}
