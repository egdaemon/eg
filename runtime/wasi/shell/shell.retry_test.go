package shell_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

// failures returns an execer which fails the first n calls.
func failures(n int) shell.Execer {
	calls := 0
	return func(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
		calls++
		if calls <= n {
			return errors.New("boom")
		}
		return nil
	}
}

func TestShellLogging(t *testing.T) {
	const (
		yellow = "\x1b[33m"
		red    = "\x1b[31m"
	)

	type example struct {
		name     string
		cmd      shell.Command
		failed   bool
		warnings int
		errors   int
		colored  bool
	}

	examples := []example{
		{
			name:     "retries warn in yellow",
			cmd:      shell.New("false").Attempts(3).UnsafeExec(failures(2)),
			warnings: 2,
			colored:  true,
		},
		{
			name:     "exhausted attempts error in red",
			cmd:      shell.New("false").Attempts(2).UnsafeExec(failures(5)),
			failed:   true,
			warnings: 1,
			errors:   1,
			colored:  true,
		},
		{
			name:    "single attempt does not warn",
			cmd:     shell.New("false").UnsafeExec(failures(1)),
			failed:  true,
			errors:  1,
			colored: true,
		},
		{
			name:     "colors disabled",
			cmd:      shell.New("false").Colors(false).Attempts(2).UnsafeExec(failures(1)),
			warnings: 1,
		},
	}

	for _, e := range examples {
		t.Run(e.name, func(t *testing.T) {
			ctx, done := testx.Context(t)
			defer done()

			var buf bytes.Buffer
			defer log.SetOutput(log.Writer())
			log.SetOutput(&buf)

			err := shell.Run(ctx, e.cmd)
			if e.failed {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			out := buf.String()
			require.Equal(t, e.warnings, strings.Count(out, "WARN"))
			require.Equal(t, e.errors, strings.Count(out, "ERROR"))
			if e.colored {
				require.Equal(t, e.warnings, strings.Count(out, yellow+"WARN"))
				require.Equal(t, e.errors, strings.Count(out, red+"ERROR"))
			} else {
				require.NotContains(t, out, "\x1b[")
			}
		})
	}
}
