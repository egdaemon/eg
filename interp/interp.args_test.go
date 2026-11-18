package interp

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestOptionArgs(t *testing.T) {
	wasm := interptest.Build(t, "./interp/.xfixtures/args")

	run := func(t *testing.T, options ...Option) string {
		ctx, done := context.WithTimeout(context.Background(), time.Minute)
		defer done()

		rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
		defer rt.Close(context.Background())

		_, err := wasi_snapshot_preview1.NewBuilder(rt).Instantiate(ctx)
		require.NoError(t, err)

		bin, err := os.ReadFile(wasm)
		require.NoError(t, err)

		ws := workspaces.Context{RuntimeDir: t.TempDir()}

		var stdout bytes.Buffer
		_, err = rt.InstantiateWithConfig(ctx, bin, newrunner(options...).modulenv(wazero.NewModuleConfig().WithStdout(&stdout).WithStderr(os.Stderr), ws, "/m.wasm"))
		require.NoError(t, err)

		return stdout.String()
	}

	t.Run("the module has the arguments", func(t *testing.T) {
		// arguments are not interpreted: spaces, empty, flags and quotes arrive as they were given.
		out := run(t, OptionArgs("a b", "", "--flag=x y", "-vv", `it's "q"`, "$HOME"))
		require.Equal(t, "\"/m.wasm\"\n\"a b\"\n\"\"\n\"--flag=x y\"\n\"-vv\"\n\"it's \\\"q\\\"\"\n\"$HOME\"\n", out)
	})

	t.Run("without arguments the module is the only one", func(t *testing.T) {
		require.Equal(t, "\"/m.wasm\"\n", run(t))
	})
}
