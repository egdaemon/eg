package runners

import (
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestStateInit(t *testing.T) {
	t.Run("creates the log and analytics before continuing to the next state", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		ws := workspaces.Context{RuntimeDir: t.TempDir()}
		next := idle(metadata{})

		result := initialize(&computeapi.Enqueued{AccountId: "acct", Id: "init"}, metadata{}, "bucket", ws, next).Update(t.Context())

		require.Equal(t, next, result)
		require.FileExists(t, filepath.Join(ws.RuntimeDir, DaemonLogName))
		require.FileExists(t, filepath.Join(ws.RuntimeDir, DaemonAnalyticsName))
	})
}
