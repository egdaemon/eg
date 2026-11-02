package egenv_test

import (
	"os"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestDraftRelease(t *testing.T) {
	t.Run("defaults to disabled", func(t *testing.T) {
		t.Setenv(eg.EnvWorkloadDraft, "")
		t.Setenv(eg.EnvWorkloadRelease, "")
		require.NoError(t, os.Unsetenv(eg.EnvWorkloadDraft))
		require.NoError(t, os.Unsetenv(eg.EnvWorkloadRelease))
		require.False(t, egenv.Draft())
		require.False(t, egenv.Release())
	})

	t.Run("draft only", func(t *testing.T) {
		t.Setenv(eg.EnvWorkloadDraft, "true")
		t.Setenv(eg.EnvWorkloadRelease, "false")
		require.True(t, egenv.Draft())
		require.False(t, egenv.Release())
	})

	t.Run("draft and release", func(t *testing.T) {
		t.Setenv(eg.EnvWorkloadDraft, "true")
		t.Setenv(eg.EnvWorkloadRelease, "true")
		require.True(t, egenv.Draft())
		require.True(t, egenv.Release())
	})

	t.Run("release without draft does not publish", func(t *testing.T) {
		t.Setenv(eg.EnvWorkloadDraft, "false")
		t.Setenv(eg.EnvWorkloadRelease, "true")
		require.False(t, egenv.Draft())
		require.False(t, egenv.Release())
	})
}
