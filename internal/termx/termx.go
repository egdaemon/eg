package termx

import (
	"errors"
	"fmt"
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

// Run runs an interactive prompt, e.g.) a huh field or form. a missing terminal is reported as ErrNoTTY wrapping the
// original error, every other error is returned untouched.
func Run(r interface{ Run() error }) error {
	err := r.Run()
	if errors.Is(err, syscall.ENXIO) || errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %w", ErrNoTTY, err)
	}

	return err
}
