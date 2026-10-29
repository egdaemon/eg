package egenv_test

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestCloneEnabled(t *testing.T) {
	t.Run("defaults to enabled", func(t *testing.T) {
		require.True(t, egenv.CloneEnabled())
	})

	t.Run("enabled", func(t *testing.T) {
		t.Setenv(eg.EnvUnsafeGitCloneEnabled, "true")
		require.True(t, egenv.CloneEnabled())
	})

	t.Run("disabled", func(t *testing.T) {
		t.Setenv(eg.EnvUnsafeGitCloneEnabled, "false")
		require.False(t, egenv.CloneEnabled())
	})
}
