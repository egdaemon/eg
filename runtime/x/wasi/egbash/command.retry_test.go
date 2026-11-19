package egbash_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/x/wasi/egbash"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// failures returns an execer which fails the first n calls.
func failures(n int) egbash.Execer {
	calls := 0
	return func(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
		calls++
		if calls <= n {
			return errors.New("boom")
		}
		return nil
	}
}

func TestRetry(t *testing.T) {
	// deploy is neither a builtin nor a coreutil, so the interpreter hands it to the execer.
	const cmd = "deploy"

	t.Run("logging", func(t *testing.T) {
		const (
			yellow = "\x1b[33m"
			red    = "\x1b[31m"
		)

		type example struct {
			name     string
			cmd      egbash.Command
			failed   bool
			warnings int
			colored  bool
		}

		examples := []example{
			{
				name:     "retries warn in yellow",
				cmd:      egbash.New(cmd).Attempts(3).UnsafeExec(failures(2)),
				warnings: 2,
				colored:  true,
			},
			{
				name:     "exhausted attempts error in red",
				cmd:      egbash.New(cmd).Attempts(2).UnsafeExec(failures(5)),
				failed:   true,
				warnings: 1,
				colored:  true,
			},
			{
				name:    "single attempt does not warn",
				cmd:     egbash.New(cmd).UnsafeExec(failures(1)),
				failed:  true,
				colored: true,
			},
			{
				name:     "lenient failures warn",
				cmd:      egbash.New(cmd).Lenient(true).UnsafeExec(failures(1)),
				warnings: 1,
				colored:  true,
			},
			{
				name:    "silent lenient failures do not warn",
				cmd:     egbash.New(cmd).Lenient(true).Silent(true).UnsafeExec(failures(1)),
				colored: true,
			},
			{
				name:    "silent retries do not warn",
				cmd:     egbash.New(cmd).Silent(true).Attempts(3).UnsafeExec(failures(2)),
				colored: true,
			},
			{
				name:    "silent does not hide a returned failure",
				cmd:     egbash.New(cmd).Silent(true).Attempts(2).UnsafeExec(failures(5)),
				failed:  true,
				colored: true,
			},
			{
				name:     "colors disabled",
				cmd:      egbash.New(cmd).Colors(false).Attempts(2).UnsafeExec(failures(1)),
				warnings: 1,
			},
		}

		for _, e := range examples {
			t.Run(e.name, func(t *testing.T) {
				var (
					buf bytes.Buffer
				)

				ctx, done := testx.Context(t)
				defer done()

				defer log.SetOutput(log.Writer())
				log.SetOutput(&buf)

				err := egbash.Run(ctx, e.cmd.Directory(t.TempDir()))
				if e.failed {
					require.Error(t, err)
					if e.colored {
						require.Contains(t, err.Error(), red+"ERROR")
					}
				} else {
					require.NoError(t, err)
				}

				out := buf.String()
				require.Equal(t, e.warnings, strings.Count(out, "WARN"))
				require.Zero(t, strings.Count(out, "ERROR"), "errors are returned, not logged")
				if e.colored {
					require.Equal(t, e.warnings, strings.Count(out, yellow+"WARN"))
				} else {
					require.NotContains(t, out, "\x1b[")
				}
			})
		}
	})

	t.Run("limiter", func(t *testing.T) {
		type example struct {
			name     string
			limiter  *rate.Limiter
			attempts int16
			failures int
			failed   bool
		}

		// a limiter that would make the second retry wait an hour, which exceeds the deadline of the context.
		hourly := func(burst int) *rate.Limiter { return rate.NewLimiter(rate.Every(time.Hour), burst) }

		examples := []example{
			{
				name:     "unlimited limiter retries without waiting",
				limiter:  rate.NewLimiter(rate.Inf, 1),
				attempts: 5,
				failures: 4,
			},
			{
				name:     "burst covers every retry",
				limiter:  hourly(2),
				attempts: 3,
				failures: 2,
			},
			{
				name:     "retries wait on the limiter",
				limiter:  hourly(1),
				attempts: 3,
				failures: 2,
				failed:   true,
			},
			{
				name:     "exhausted attempts return the failure",
				limiter:  rate.NewLimiter(rate.Inf, 1),
				attempts: 2,
				failures: 5,
				failed:   true,
			},
		}

		for _, e := range examples {
			t.Run(e.name, func(t *testing.T) {
				ctx, done := testx.Context(t)
				defer done()

				ctx, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()

				err := egbash.Run(ctx, egbash.New(cmd).Directory(t.TempDir()).Attempts(e.attempts).Retry(e.limiter).UnsafeExec(failures(e.failures)))
				if e.failed {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}
