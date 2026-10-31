package termx

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsInteractive true when prompting is possible.
func IsInteractive() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
}
