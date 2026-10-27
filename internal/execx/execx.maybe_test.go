package execx

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaybeRun(t *testing.T) {
	t.Run("nil command is a noop", func(t *testing.T) {
		require.NoError(t, MaybeRun(nil))
	})

	t.Run("successful command", func(t *testing.T) {
		require.NoError(t, MaybeRun(exec.CommandContext(t.Context(), "true")))
	})

	t.Run("failing command returns the error", func(t *testing.T) {
		require.Error(t, MaybeRun(exec.CommandContext(t.Context(), "false")))
	})
}
