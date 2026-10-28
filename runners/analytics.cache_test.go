package runners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestCacheAnalytics(t *testing.T) {
	setup := func(t *testing.T, requested bool) (workspaces.Context, string) {
		ws := workspaces.Context{RuntimeDir: t.TempDir(), CacheDir: t.TempDir()}
		src := filepath.Join(ws.RuntimeDir, DaemonAnalyticsName)
		require.NoError(t, os.WriteFile(src, []byte("analytics"), 0600))
		if requested {
			require.NoError(t, os.WriteFile(filepath.Join(ws.RuntimeDir, eg.AnalyticsCacheRequest), nil, 0600))
		}
		return ws, src
	}

	t.Run("copies when requested", func(t *testing.T) {
		ws, src := setup(t, true)
		require.NoError(t, cacheanalytics(ws, "main.wasm", src))

		content, err := os.ReadFile(filepath.Join(ws.CacheDir, eg.AnalyticsCachePath("main.wasm")))
		require.NoError(t, err)
		require.Equal(t, "analytics", string(content))
	})

	t.Run("noop when not requested", func(t *testing.T) {
		ws, src := setup(t, false)
		require.NoError(t, cacheanalytics(ws, "main.wasm", src))
		require.NoFileExists(t, filepath.Join(ws.CacheDir, eg.AnalyticsCachePath("main.wasm")))
	})
}
