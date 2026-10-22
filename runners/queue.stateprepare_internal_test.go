package runners

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestStatePrepare(t *testing.T) {
	t.Run("failure is appended to the logs and completes the workload with the cause", func(t *testing.T) {
		ws := workspaces.Context{RuntimeDir: t.TempDir()}
		require.NoError(t, os.WriteFile(filepath.Join(ws.RuntimeDir, DaemonLogName), []byte("existing\n"), 0600))
		cause := errorsx.Wrap(errors.ErrUnsupported, "unable to configure gpu support")

		result := stateprepare{
			workload: &EnqueuedDequeueResponse{Enqueued: &Enqueued{Id: "prepare"}},
			ws:       ws,
		}.failed(cause)

		completed, ok := result.(statecompleted)
		require.True(t, ok)
		require.ErrorIs(t, completed.cause, errors.ErrUnsupported)

		logs, err := os.ReadFile(filepath.Join(ws.RuntimeDir, DaemonLogName))
		require.NoError(t, err)
		require.Contains(t, string(logs), "existing")
		require.Contains(t, string(logs), "unable to configure gpu support")
	})
}
