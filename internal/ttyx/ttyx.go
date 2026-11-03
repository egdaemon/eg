// Package ttyx separates what is piped into the process from the terminal the user types on, for commands
// that consume stdin as data and still need to hand the user an interactive program, e.g.) an editor.
package ttyx

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/egdaemon/eg"
	"github.com/mattn/go-isatty"
)

// replaced by tests. stdin is a function because tests, and callers, swap os.Stdin.
var (
	stdin   = func() *os.File { return os.Stdin }
	ttypath = defaultpath()
)

func defaultpath() string {
	if runtime.GOOS == "windows" {
		return "CONIN$"
	}

	return "/dev/tty"
}

// override is the terminal device chosen by the caller, e.g.) a flag. it takes precedence over the environment.
var override string

// SetPath selects the terminal device to use instead of the default, an empty path clears the selection.
// a selected device, from here or the environment, is the only one consulted: stdin being a terminal
// no longer counts, which means a path that does not exist is how to say there is no terminal.
func SetPath(p string) {
	override = p
}

// devicepath is the terminal device to open and whether it was chosen rather than defaulted.
func devicepath() (string, bool) {
	if override != "" {
		return override, true
	}

	if p := os.Getenv(eg.EnvTTY); p != "" {
		return p, true
	}

	return ttypath, false
}

func isterminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// Read drains stdin when it holds piped input. when stdin is a terminal the user is going to type
// rather than pipe, so nothing is read and the result is empty: Input is where that terminal is found.
// reading stops with the error of the context when it is done, Close releases the reader early.
func Read(ctx context.Context) io.ReadCloser {
	in := stdin()
	if isterminal(in) {
		return io.NopCloser(strings.NewReader(""))
	}

	pr, pw := io.Pipe()
	go func() {
		_, err := io.Copy(pw, in)
		pw.CloseWithError(err)
	}()

	d := &drain{PipeReader: pr, ctx: ctx, stop: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			pr.CloseWithError(context.Cause(ctx))
		case <-d.stop:
		}
	}()

	return d
}

type drain struct {
	*io.PipeReader
	ctx  context.Context
	stop chan struct{}
	once sync.Once
}

// Read reports the context as the reason when it is what closed the pipe.
func (t *drain) Read(p []byte) (int, error) {
	n, err := t.PipeReader.Read(p)
	if errors.Is(err, io.ErrClosedPipe) && t.ctx.Err() != nil {
		return n, context.Cause(t.ctx)
	}

	return n, err
}

func (t *drain) Close() error {
	t.once.Do(func() { close(t.stop) })
	return t.PipeReader.Close()
}

// Input returns the terminal the user types on, for handing to an interactive program as its stdin.
// it is stdin when that is a terminal, otherwise the controlling terminal of the process. without a
// terminal at all (cron, ci) it falls back to stdin so scripted programs keep working. the result is
// an *os.File whenever the platform allows, which is what lets a program run by os/exec see the terminal,
// closing it never closes stdin.
func Input(ctx context.Context) io.ReadCloser {
	in := stdin()
	path, chosen := devicepath()
	if !chosen && isterminal(in) {
		if f, err := dup(in); err == nil {
			return f
		}
	}

	if tty, err := os.Open(path); err == nil {
		return tty
	}

	if f, err := dup(in); err == nil {
		return f
	}

	return io.NopCloser(in)
}

// Available reports whether there is a terminal for the user to type on: stdin when that is a terminal,
// otherwise the controlling terminal of the process. it is false for cron, ci and the like, where
// Input would fall back to stdin and an interactive program has nobody to talk to.
func Available() bool {
	path, chosen := devicepath()
	if !chosen && isterminal(stdin()) {
		return true
	}

	tty, err := os.Open(path)
	if err != nil {
		return false
	}
	defer tty.Close()

	return true
}
