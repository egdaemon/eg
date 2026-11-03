// Package cmdtestx provides a harness for exercising kong-based CLI commands
// entirely in-process, without shelling out to a built binary.
package cmdtestx

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/contextx"
)

// Harness runs a command through a kong parser. nothing is built until Run, so every
// kong.Option (including kong.Vars) applies.
type Harness struct {
	t       *testing.T
	cmd     any
	global  cmdopts.Global
	environ []string
	opts    []kong.Option
}

// New creates a harness for cmd, which must be a pointer to a kong command struct. the
// cmdopts.Global is created and bound for the command; its context is cancelled and
// its cleanup waited on when the test completes.
func New(t *testing.T, cmd any) *Harness {
	t.Helper()

	h := &Harness{t: t, cmd: cmd}
	h.global.Cleanup = &sync.WaitGroup{}
	h.global.Context, h.global.Shutdown = context.WithCancelCause(contextx.WithWaitGroup(context.Background(), h.global.Cleanup))

	t.Cleanup(func() {
		h.global.Shutdown(nil)
		h.global.Cleanup.Wait()
	})

	return h
}

// Env provides environment variables, in the form KEY=value, to the command without touching
// the process environment. flags declaring `env:"NAME"` are populated from them when NAME is
// present. a value given on the command line wins, otherwise these win over the flag's default
// and the process environment. when a key is repeated, the last value wins. positional
// arguments don't support env tags and aren't affected.
func (t *Harness) Env(environ ...string) *Harness {
	t.environ = append(t.environ, environ...)
	return t
}

// Options adds kong options used to build the parser, e.g. kong.Vars or kong.Bind.
func (t *Harness) Options(opts ...kong.Option) *Harness {
	t.opts = append(t.opts, opts...)
	return t
}

// Run builds the parser, parses args and runs the resolved command.
func (t *Harness) Run(args ...string) error {
	t.t.Helper()

	lookup := make(map[string]string, len(t.environ))
	for _, kv := range t.environ {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return fmt.Errorf("malformed environment variable %q, expected KEY=value", kv)
		}

		lookup[k] = v
	}

	resolver := kong.ResolverFunc(func(_ *kong.Context, _ *kong.Path, flag *kong.Flag) (any, error) {
		for _, name := range flag.Tag.Envs {
			if v, ok := lookup[name]; ok {
				return v, nil
			}
		}

		return nil, nil
	})

	opts := []kong.Option{
		kong.Vars{"env_eg_account": eg.EnvAccount, "env_eg_profile": eg.EnvProfile},
		kong.Bind(&t.global),
	}
	opts = append(opts, t.opts...)
	parser, err := kong.New(t.cmd, append(opts, kong.Resolvers(resolver))...)
	if err != nil {
		return err
	}

	kctx, err := parser.Parse(args)
	if err != nil {
		return err
	}

	return kctx.Run()
}
