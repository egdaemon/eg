package egenv_test

import (
	"os"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestCloneEnabled(t *testing.T) {
	t.Run("defaults to enabled", func(t *testing.T) {
		// eg compute local sets the variable to false, ensure it is unset for this case.
		t.Setenv(eg.EnvUnsafeGitCloneEnabled, "")
		require.NoError(t, os.Unsetenv(eg.EnvUnsafeGitCloneEnabled))
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
