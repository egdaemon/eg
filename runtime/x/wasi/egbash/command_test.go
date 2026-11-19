package egbash_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/x/wasi/egbash"
	"github.com/egdaemon/sh/v3/interp"
	"github.com/stretchr/testify/require"
)

func TestNewRecorder(t *testing.T) {
	type example struct {
		name     string
		cmd      func(dir string) egbash.Command
		expected func(dir string) []string
	}

	examples := []example{
		{
			name: "records the external command and its arguments",
			cmd:  func(dir string) egbash.Command { return egbash.New("deploy --to prod").Directory(dir) },
			expected: func(dir string) []string {
				return []string{dir + "::deploy:--to prod"}
			},
		},
		{
			name: "records only the variables added to the environment",
			cmd: func(dir string) egbash.Command {
				return egbash.Env().Environ("EGBASH_TEST", "derp").New("deploy").Directory(dir)
			},
			expected: func(dir string) []string {
				return []string{dir + ":EGBASH_TEST=derp:deploy:"}
			},
		},
		{
			name: "records variables the command exports",
			cmd: func(dir string) egbash.Command {
				return egbash.New("export EGBASH_TEST=derp; deploy").Directory(dir)
			},
			expected: func(dir string) []string {
				return []string{dir + ":EGBASH_TEST=derp:deploy:"}
			},
		},
		{
			name: "records each command a script hands off, after expansion",
			cmd: func(dir string) egbash.Command {
				return egbash.New("for v in a b; do deploy $v; done").Directory(dir)
			},
			expected: func(dir string) []string {
				return []string{dir + "::deploy:a", dir + "::deploy:b"}
			},
		},
		{
			name: "builtins and coreutils stay in the interpreter",
			cmd: func(dir string) egbash.Command {
				return egbash.New("echo hello > out; cat out > /dev/null").Directory(dir)
			},
			expected: func(dir string) []string {
				return nil
			},
		},
	}

	for _, e := range examples {
		t.Run(e.name, func(t *testing.T) {
			ctx, done := testx.Context(t)
			defer done()

			dir := t.TempDir()
			cmd := e.cmd(dir)
			rec := egbash.NewRecorder(&cmd)
			require.NoError(t, egbash.Run(ctx, cmd))
			require.Equal(t, e.expected(dir), rec.Results())
		})
	}
}

func TestNewLocal(t *testing.T) {
	t.Run("runs external commands as processes", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		dir := t.TempDir()
		require.NoError(t, egbash.Run(ctx, egbash.NewLocal().Environ("EGBASH_TEST", "derp").Directory(dir).New(`/bin/sh -c 'echo "$EGBASH_TEST"' | tr a-z A-Z > out`)))
		b, err := os.ReadFile(filepath.Join(dir, "out"))
		require.NoError(t, err)
		require.Equal(t, "DERP\n", string(b))
	})

	t.Run("a non-zero exit reaches ||", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		dir := t.TempDir()
		require.NoError(t, egbash.Run(ctx, egbash.NewLocal().Directory(dir).New(`/bin/sh -c 'exit 3' || echo fallback > out`)))
		b, err := os.ReadFile(filepath.Join(dir, "out"))
		require.NoError(t, err)
		require.Equal(t, "fallback\n", string(b))
	})

	t.Run("a non-zero exit is returned as the exit status", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		err := egbash.Run(ctx, egbash.NewLocal().Directory(t.TempDir()).New(`/bin/sh -c 'exit 3'`))
		status, ok := errors.AsType[interp.ExitStatus](err)
		require.True(t, ok, err)
		require.Equal(t, interp.ExitStatus(3), status)
	})
}
