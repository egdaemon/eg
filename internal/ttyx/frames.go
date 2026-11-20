package ttyx

import (
	"encoding/binary"
	"errors"
	"io"
)

// the frames spoken between a Host and the clients attached to it: a type, the length of the payload as 4 bytes, the payload.
const (
	frameData   byte = 'd' // bytes of the terminal: output from the host, key presses from a client.
	frameResize byte = 'r' // from a client: its window size, rows then columns as 2 bytes each.
)

// maxframe bounds what a peer can make the other side allocate.
const maxframe = 4 << 20

func writeFrame(w io.Writer, kind byte, payload []byte) error {
	header := make([]byte, 5, 5+len(payload))
	header[0] = kind
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))

	// one write so frames from different goroutines are not interleaved by the caller's lock alone being forgotten.
	_, err := w.Write(append(header, payload...))
	return err
}

func readFrame(r io.Reader) (kind byte, payload []byte, err error) {
	header := make([]byte, 5)
	if _, err = io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}

	n := binary.BigEndian.Uint32(header[1:])
	if n > maxframe {
		return 0, nil, errors.New("terminal frame too large")
	}

	payload = make([]byte, n)
	if _, err = io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}

	return header[0], payload, nil
}

func encodeSize(rows, cols uint16) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint16(b, rows)
	binary.BigEndian.PutUint16(b[2:], cols)
	return b
}

func decodeSize(b []byte) (rows, cols uint16, ok bool) {
	if len(b) != 4 {
		return 0, 0, false
	}

	return binary.BigEndian.Uint16(b), binary.BigEndian.Uint16(b[2:]), true
}
