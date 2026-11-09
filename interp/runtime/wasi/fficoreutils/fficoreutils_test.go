package fficoreutils_test

import (
	"context"
	"testing"

	"github.com/egdaemon/eg/interp/runtime/wasi/fficoreutils"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func TestUnavailable(t *testing.T) {
	runtime := func(t *testing.T) wazero.Runtime {
		rt := wazero.NewRuntime(t.Context())
		t.Cleanup(func() { rt.Close(context.Background()) })
		return rt
	}

	// the egbash guest tests run commands through the stand in, so this only checks what guests import.
	t.Run("provides the functions guests import when the runtime has no reactor", func(t *testing.T) {
		rt := runtime(t)
		m, err := fficoreutils.Unavailable(t.Context(), rt)
		require.NoError(t, err)
		defer m.Close(t.Context())

		definitions := rt.Module(fficoreutils.Name).ExportedFunctionDefinitions()
		for _, fn := range []string{"execveatinit", "execveatwait"} {
			require.Contains(t, definitions, fn)
			require.Equal(t, []api.ValueType{api.ValueTypeI64}, definitions[fn].ParamTypes(), fn)
			require.Equal(t, []api.ValueType{api.ValueTypeI32}, definitions[fn].ResultTypes(), fn)
		}
	})

	t.Run("leaves a loaded reactor in place", func(t *testing.T) {
		rt := runtime(t)
		reactor, err := rt.NewHostModuleBuilder(fficoreutils.Name).
			NewFunctionBuilder().WithFunc(func(ctx context.Context, id uint64) uint32 { return 0 }).Export("execveatinit").
			Instantiate(t.Context())
		require.NoError(t, err)

		m, err := fficoreutils.Unavailable(t.Context(), rt)
		require.NoError(t, err)
		require.NoError(t, m.Close(t.Context()))

		require.Equal(t, reactor, rt.Module(fficoreutils.Name))
		require.False(t, reactor.IsClosed())
	})
}
