package eggolang

import (
	"testing"

	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestRuntime(t *testing.T) {
	t.Run("coveragedir is ephemeral", func(t *testing.T) {
		require.Equal(t, egenv.EphemeralDirectory(".eg.coverage"), coveragedir())
	})

	t.Run("Runtime carries the golang environment", func(t *testing.T) {
		cmd := Runtime()
		rec := shell.NewRecorder(&cmd)

		require.NoError(t, shell.Run(t.Context(), cmd.New("true")))
		require.Contains(t, rec.Result(), "GOCACHE="+CacheBuildDirectory())
	})
}
