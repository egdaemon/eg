package testx

import (
	"log"
	"os"
	"testing"

	"github.com/mattn/go-isatty"
)

// Logging enable logging if stdout terminal is a tty.
// generally this means run the ginkgo without the -p (parallel) option.
func Logging() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.LUTC)
	log.SetOutput(os.Stderr)

	if isatty.IsTerminal(os.Stdout.Fd()) {
		return
	}

	// log.SetOutput(io.Discard)
}

func PrivateTemp(t testing.TB) string {
	dst := t.TempDir()
	t.Setenv("TMPDIR", dst)
	return dst
}
