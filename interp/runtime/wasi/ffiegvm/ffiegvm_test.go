package ffiegvm_test

import (
	"context"
	"os"
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiegvm"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestRun(t *testing.T) {
	type ran struct {
		location   string
		toc        string
		modulepath string
		environ    []string
	}

	wasm := interptest.Build(t, "./interp/runtime/wasi/ffiegvm/.xfixtures/guest")

	// run runs the guest with runner behind the host function, returning its error.
	run := func(t *testing.T, runner func(ctx context.Context, location string, toc string, modulepath string, environ ...string) error) error {
		ctx := t.Context()
		rt := wazero.NewRuntime(ctx)
		defer rt.Close(context.Background())
		wasi_snapshot_preview1.MustInstantiate(ctx, rt)

		_, err := rt.NewHostModuleBuilder("env").
			NewFunctionBuilder().WithFunc(ffiegvm.Run(runner)).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegvm.Run").
			Instantiate(ctx)
		require.NoError(t, err)

		bin, err := os.ReadFile(wasm)
		require.NoError(t, err)
		_, err = rt.InstantiateWithConfig(ctx, bin, wazero.NewModuleConfig().WithStderr(os.Stderr))
		return err
	}

	t.Run("passes the guest's arguments to the runner", func(t *testing.T) {
		var got []ran
		require.NoError(t, run(t, func(ctx context.Context, location string, toc string, modulepath string, environ ...string) error {
			got = append(got, ran{location: location, toc: toc, modulepath: modulepath, environ: environ})
			return nil
		}))
		require.Equal(t, []ran{{location: "https://example.com/root.egiso", toc: "sha256:abc", modulepath: "module.wasm", environ: []string{"A=1", "B=2"}}}, got)
	})

	t.Run("reports the runner's failure to the guest", func(t *testing.T) {
		require.Error(t, run(t, func(ctx context.Context, location string, toc string, modulepath string, environ ...string) error {
			return errorsx.New("boom")
		}))
	})
}
