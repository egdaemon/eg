// Package cmdtestx provides a shared harness for exercising kong-based CLI commands
// entirely in-process, without shelling out to a built binary.
package cmdtestx

import (
	"context"
	"sync"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
)

// Genparser returns a function that builds a *kong.Kong parser for the given leaf command,
// bound with cmdopts.Global the same way cmd/eg/main.go binds it. Additional kong.Option
// values (e.g. kong.Vars{...} to stand in for values normally sourced from the process
// environment) can be supplied by the caller.
func Genparser[T any](cmd T, opts ...kong.Option) func(t *testing.T) *kong.Kong {
	return func(t *testing.T) *kong.Kong {
		t.Helper()

		var cli struct {
			cmdopts.Global
			Command T `cmd:""`
		}

		cli.Context, cli.Shutdown = context.WithCancelCause(context.Background())
		cli.Cleanup = &sync.WaitGroup{}

		return kong.Must(&cli, append(opts, kong.Bind(&cli.Global))...)
	}
}

// Execute parses args against the given parser and, if parsing succeeds, runs the resolved
// command.
func Execute(t *testing.T, parser *kong.Kong, args ...string) error {
	t.Helper()

	kctx, err := parser.Parse(args)
	if err != nil {
		return err
	}

	return kctx.Run()
}
