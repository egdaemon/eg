// guest used by the shell tests: interprets commands with Run inside the wasi runtime and exits non-zero
// when their result is wrong.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell/v2"
	"github.com/egdaemon/sh/v3/interp"
)

// SHELL_MODE selects the check:
//
//	pipeline     - a builtin piped into a host command, redirected to a file in the guest.
//	substitution - the output of host commands substituted into guest arithmetic.
//	devnull      - /dev/null, which the guest has no mount for, and a failing host command reaching ||.
//	exit         - a host command's exit code is returned as the interpreter's exit status.
func main() {
	var (
		err error
	)

	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	expect := func(script string, out string) {
		var (
			b []byte
		)
		if err = shell.Run(ctx, "", "", script, "", nil); err != nil {
			log.Fatalln("run failed", script, err)
		}
		if b, err = os.ReadFile(egenv.WorkingDirectory("out")); err != nil {
			log.Fatalln("unable to read the output", err)
		}
		if string(b) != out {
			log.Fatalf("unexpected output %q, expected %q\n", b, out)
		}
	}

	switch mode := os.Getenv("SHELL_MODE"); mode {
	case "pipeline":
		expect("echo hello | tr a-z A-Z > out", "HELLO\n")
	case "substitution":
		expect(`n=$(printf abc | wc -c); echo $((n + 1)) > out`, "4\n")
	case "devnull":
		expect("echo hidden > /dev/null; ls does-not-exist 2>/dev/null || echo fallback > out", "fallback\n")
	case "exit":
		err = shell.Run(ctx, "", "", "sh -c 'exit 3'", "", nil)
		if status, ok := errors.AsType[interp.ExitStatus](err); !ok || status != 3 {
			log.Fatalln("unexpected result", err)
		}
	default:
		log.Fatalln("unknown mode", mode)
	}
}
