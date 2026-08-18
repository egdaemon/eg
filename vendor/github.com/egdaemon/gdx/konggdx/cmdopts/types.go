// Package cmdopts holds small reusable kong flag types for konggdx commands,
// mirroring the equivalent types in this ecosystem's other CLIs (e.g.
// shallows/cmd/cmdopts) without depending on them.
package cmdopts

import (
	"io"
	"os"
)

// IOOut is a flag type that opens a file for writing, or uses stdout when the path is "-".
type IOOut struct {
	path string
}

func (t *IOOut) UnmarshalText(text []byte) error {
	t.path = string(text)
	return nil
}

func (t IOOut) MarshalText() ([]byte, error) {
	return []byte(t.path), nil
}

// Open returns a WriteCloser for the output. When path is "-", writes go to
// fallback and Close is a no-op. Otherwise a new file is created (or truncated).
func (t IOOut) Open(fallback io.Writer) (io.WriteCloser, error) {
	if t.path == "-" {
		return nopWriteCloser{fallback}, nil
	}
	return os.OpenFile(t.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
