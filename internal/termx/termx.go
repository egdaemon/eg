package termx

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/mattn/go-isatty"
)

// ErrNoTTY signals that a prompt was requested but no terminal is available.
const ErrNoTTY = errorsx.String("no terminal available")

// IsInteractive true when prompting is possible.
func IsInteractive() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
}

// IsTTY true when every one of vs is a terminal, i.e.) it has a file descriptor, an *os.File, that is one. an io.Reader or
// io.Writer that is anything else, or nil, is not. there is nothing to be a terminal when none are given.
func IsTTY(vs ...any) bool {
	for _, v := range vs {
		if f, ok := v.(interface{ Fd() uintptr }); !ok || !isatty.IsTerminal(f.Fd()) {
			return false
		}
	}

	return len(vs) > 0
}

// Run runs an interactive prompt, e.g.) a huh field or form. a missing terminal is reported as ErrNoTTY wrapping the
// original error, every other error is returned untouched.
func Run(r interface{ Run() error }) error {
	err := r.Run()
	if errors.Is(err, syscall.ENXIO) || errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %w", ErrNoTTY, err)
	}

	return err
}

type hangupEOF struct {
	io.Reader
}

func (t hangupEOF) Read(p []byte) (int, error) {
	n, err := t.Reader.Read(p)
	if errors.Is(err, syscall.EIO) {
		return n, io.EOF
	}

	return n, err
}

// HangupEOF wraps r such that, when r is a terminal, the EIO a pty returns once its controlling terminal hangs up, e.g.)
// the window was closed, is reported as io.EOF; a normal end of input. anything that is not a terminal is returned untouched.
func HangupEOF(r io.Reader) io.Reader {
	if !IsTTY(r) {
		return r
	}

	return hangupEOF{Reader: r}
}
