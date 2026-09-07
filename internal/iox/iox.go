package iox

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
)

// IgnoreEOF returns nil if err is io.EOF
func IgnoreEOF(err error) error {
	if errorsx.Cause(err) != io.EOF {
		return err
	}

	return nil
}

// Error return just the error from an IO call ignoring the number of bytes.
func Error(_ int64, err error) error {
	return err
}

// Rewind an io.Seeker
func Rewind(o io.Seeker) error {
	_, err := o.Seek(0, io.SeekStart)
	return err
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

// Copy a file to another path
func Copy(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()

	i, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, i.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Close()
}

func MaybeClose(c io.Closer) error {
	if c == nil || c == (*os.File)(nil) {
		return nil
	}

	return c.Close()
}

type zreader struct{}

func (z *zreader) Read(p []byte) (n int, err error) {
	// Return zero bytes read and no error
	return 0, nil
}

func Zero() io.Reader {
	return &zreader{}
}

func String(r io.Reader) string {
	defer func() {
		if x, ok := r.(io.Seeker); ok {
			_ = Rewind(x)
		}
	}()

	raw, _ := io.ReadAll(r)

	return string(raw)
}

type readCompositeCloser struct {
	io.Reader
	closefn []func() error
}

func (t readCompositeCloser) Close() (err error) {
	for _, fn := range t.closefn {
		err = errorsx.Compact(err, fn())
	}
	return err
}

// WriteNopCloser returns a WriteCloser with a no-op Close method wrapping
// the provided Writer w.
func ReaderCompositeCloser(w io.Reader, closers ...func() error) io.ReadCloser {
	return readCompositeCloser{Reader: w, closefn: closers}
}

func TimeoutReader(d time.Duration, s io.ReadCloser) *timeoutreader {
	return newTimeoutReader(d, s)
}

func newTimeoutReader(d time.Duration, r io.ReadCloser) *timeoutreader {
	return &timeoutreader{
		inner: r,
		d:     d,
		timer: time.NewTimer(d),
	}
}

type timeoutreader struct {
	inner io.ReadCloser
	d     time.Duration
	timer *time.Timer
}

func (t *timeoutreader) Read(b []byte) (n int, err error) {
	n, err = t.inner.Read(b)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}

	select {
	case <-t.timer.C:
		return 0, context.DeadlineExceeded
	default:
		t.timer.Reset(t.d)
	}

	return n, err
}

func (t *timeoutreader) Close() error {
	return t.inner.Close()
}

func DelayReader(d time.Duration, s io.ReadCloser) *delayreader {
	return &delayreader{
		inner: s,
		d:     d,
	}
}

type delayreader struct {
	inner io.ReadCloser
	d     time.Duration
}

func (t *delayreader) Read(b []byte) (n int, err error) {
	n, err = t.inner.Read(b)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}

	time.Sleep(t.d)

	return n, err
}

func (t *delayreader) Close() error {
	return t.inner.Close()
}
