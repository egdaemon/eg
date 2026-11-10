package cmdopts

import (
	"io"
	"os"
)

// IOIn is a flag type that opens a file for reading, or uses stdin when the path is "-". it is IOOut for input.
type IOIn struct {
	path string
}

func (t *IOIn) UnmarshalText(text []byte) error {
	t.path = string(text)
	return nil
}

func (t IOIn) MarshalText() ([]byte, error) {
	return []byte(t.path), nil
}

// Open returns a ReadCloser for the input. When path is "-", reads come from
// fallback and Close is a no-op. Otherwise the file is opened.
func (t IOIn) Open(fallback io.Reader) (io.ReadCloser, error) {
	if t.path == "-" {
		return io.NopCloser(fallback), nil
	}
	return os.Open(t.path)
}
