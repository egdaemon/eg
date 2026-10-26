package egllm_test

import (
	"context"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestWith(t *testing.T) {
	t.Run("runs the operation directly when the endpoint is overridden", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeLLMEndpoint, "http://llm.internal:9090")

		var ran bool
		op := eg.OpFn(func(ctx context.Context, _ eg.Op) error {
			ran = true
			return nil
		})

		// managing a local llama-server needs the shell runtime, which fails outside of a workload.
		require.NoError(t, egllm.With("model", op)(t.Context(), egtest.Op()))
		require.True(t, ran)
	})
}
