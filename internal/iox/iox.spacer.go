package iox

import (
	"bytes"
	"errors"
	"io"

	"github.com/egdaemon/eg/internal/errorsx"
)

// Tail remembers the last bytes written to it, e.g.) with io.TeeReader, to hand to SpacerFrom.
type Tail struct {
	size uint
	last []byte
}

// NewTail remembers up to the last size bytes written.
func NewTail(size uint) *Tail {
	return &Tail{size: size}
}

func (t *Tail) Write(p []byte) (int, error) {
	t.last = append(t.last, p...)
	if uint(len(t.last)) > t.size {
		t.last = append(t.last[:0], t.last[uint(len(t.last))-t.size:]...)
	}

	return len(p), nil
}

// Last is the most recent bytes written, at most the size given to NewTail and empty until something has been.
func (t *Tail) Last() []byte {
	return t.last
}

// SpacerFrom reads a single byte from r to find out whether it holds any data. prev is the tail
// of what came before r, empty when there was nothing.
//
//	r supplied a byte -> that byte and the rest of r, preceded by a newline unless prev is empty or ends in one.
//	                     the newline keeps the data from joining whatever came before it.
//	r is empty        -> r, nothing is added.
//	r failed          -> a reader that returns the error.
//
// only the one byte is read, the rest of r is not buffered.
func SpacerFrom(prev []byte, r io.Reader) io.Reader {
	var buf = [...]byte{'\n', 0}
	if _, err := io.ReadFull(r, buf[1:]); err != nil {
		if errors.Is(err, io.EOF) {
			return r
		}

		return errorsx.Reader(err)
	}

	if len(prev) == 0 || prev[len(prev)-1] == '\n' {
		return io.MultiReader(bytes.NewReader(buf[1:]), r)
	}

	return io.MultiReader(bytes.NewReader(buf[:]), r)
}
