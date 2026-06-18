package iox

import (
	"io"
)

// Rewind the io.Seeker
func Rewind(i io.Seeker) (err error) {
	_, err = i.Seek(0, io.SeekStart)
	return err
}

// Error discards the byte count and returns just the error.
func Error(_ int64, err error) error {
	return err
}

// ReadString reads the entire string from a reader.
// if the reader is also a seeker it'll rewind before reading.
func ReadString(in io.Reader) (s string, err error) {
	var (
		raw []byte
	)

	if seeker, ok := in.(io.Seeker); ok {
		if err = Rewind(seeker); err != nil {
			return "", err
		}
	}

	if raw, err = io.ReadAll(in); err != nil {
		return "", err
	}

	return string(raw), nil
}

type NoopWriteCloser struct {
	io.WriteCloser
}

func (t NoopWriteCloser) Close() error {
	return nil
}

type writeNopCloser struct {
	io.Writer
}

func (writeNopCloser) Close() error { return nil }

// WriteNopCloser returns a WriteCloser with a no-op Close method wrapping
// the provided Writer w.
func WriteNopCloser(w io.Writer) io.WriteCloser {
	return writeNopCloser{w}
}
