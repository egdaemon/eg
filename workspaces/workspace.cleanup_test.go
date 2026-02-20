package workspaces_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func touch(t *testing.T, path string, age time.Duration) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte{}, 0644))
	ts := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(path, ts, ts))
}

func TestCleanup(t *testing.T) {
	t.Run("cache_removes_entries_older_than_30_days", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		cacheDir := t.TempDir()
		ws := workspaces.Context{CacheDir: cacheDir}
		touch(t, filepath.Join(cacheDir, "old", ".keep"), 31*24*time.Hour)
		touch(t, filepath.Join(cacheDir, "recent", ".keep"), 1*time.Hour)

		ws.Cleanup(ctx)

		_, err := os.Stat(filepath.Join(cacheDir, "old"))
		require.ErrorIs(t, err, os.ErrNotExist, "old entry should be removed")
		_, err = os.Stat(filepath.Join(cacheDir, "recent"))
		require.NoError(t, err, "recent entry should be kept")
	})

	t.Run("cache_keeps_recent_entries", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		cacheDir := t.TempDir()
		ws := workspaces.Context{CacheDir: cacheDir}
		touch(t, filepath.Join(cacheDir, "a", ".keep"), 1*time.Hour)
		touch(t, filepath.Join(cacheDir, "b", ".keep"), 29*24*time.Hour)

		ws.Cleanup(ctx)

		_, err := os.Stat(filepath.Join(cacheDir, "a"))
		require.NoError(t, err, "entry a should be kept")
		_, err = os.Stat(filepath.Join(cacheDir, "b"))
		require.NoError(t, err, "entry b should be kept")
	})

	t.Run("wazero_keeps_newest_3", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		wazeroCacheDir := t.TempDir()
		ws := workspaces.Context{CacheDirWazero: wazeroCacheDir}
		touch(t, filepath.Join(wazeroCacheDir, "e1", ".keep"), 5*time.Hour)
		touch(t, filepath.Join(wazeroCacheDir, "e2", ".keep"), 4*time.Hour)
		touch(t, filepath.Join(wazeroCacheDir, "e3", ".keep"), 3*time.Hour)
		touch(t, filepath.Join(wazeroCacheDir, "e4", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(wazeroCacheDir, "e5", ".keep"), 1*time.Hour)

		ws.Cleanup(ctx)

		_, err := os.Stat(filepath.Join(wazeroCacheDir, "e1"))
		require.ErrorIs(t, err, os.ErrNotExist, "oldest wazero entry should be removed")
		_, err = os.Stat(filepath.Join(wazeroCacheDir, "e2"))
		require.ErrorIs(t, err, os.ErrNotExist, "second oldest wazero entry should be removed")
		_, err = os.Stat(filepath.Join(wazeroCacheDir, "e3"))
		require.NoError(t, err, "e3 should be kept")
		_, err = os.Stat(filepath.Join(wazeroCacheDir, "e4"))
		require.NoError(t, err, "e4 should be kept")
		_, err = os.Stat(filepath.Join(wazeroCacheDir, "e5"))
		require.NoError(t, err, "e5 should be kept")
	})

	t.Run("gen_keeps_newest_3", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		cacheDir := t.TempDir()
		ws := workspaces.Context{CacheDir: cacheDir}
		genDir := filepath.Join(cacheDir, eg.DefaultModuleDirectory(), ".gen")
		touch(t, filepath.Join(genDir, "e1", ".keep"), 5*time.Hour)
		touch(t, filepath.Join(genDir, "e2", ".keep"), 4*time.Hour)
		touch(t, filepath.Join(genDir, "e3", ".keep"), 3*time.Hour)
		touch(t, filepath.Join(genDir, "e4", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(genDir, "e5", ".keep"), 1*time.Hour)

		ws.Cleanup(ctx)

		_, err := os.Stat(filepath.Join(genDir, "e1"))
		require.ErrorIs(t, err, os.ErrNotExist, "oldest gen entry should be removed")
		_, err = os.Stat(filepath.Join(genDir, "e2"))
		require.ErrorIs(t, err, os.ErrNotExist, "second oldest gen entry should be removed")
		_, err = os.Stat(filepath.Join(genDir, "e3"))
		require.NoError(t, err, "e3 should be kept")
		_, err = os.Stat(filepath.Join(genDir, "e4"))
		require.NoError(t, err, "e4 should be kept")
		_, err = os.Stat(filepath.Join(genDir, "e5"))
		require.NoError(t, err, "e5 should be kept")
	})

	t.Run("cache_levels_limits_depth_traversal", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		cacheDir := t.TempDir()
		ws := workspaces.Context{CacheDir: cacheDir}
		// recent file within scan range - this is what Cleanup sees
		touch(t, filepath.Join(cacheDir, "candidate", "recent.txt"), 1*time.Hour)
		// old file inside a directory at depth 8 from root; that dir gets SkipDir'd so old.txt is never visited
		touch(t, filepath.Join(cacheDir, "candidate", "1", "2", "3", "4", "5", "6", "7", "old.txt"), 31*24*time.Hour)

		ws.Cleanup(ctx)

		_, err := os.Stat(filepath.Join(cacheDir, "candidate"))
		require.NoError(t, err, "candidate should be kept: old file is beyond scan depth")
	})

	t.Run("empty_dirs_handled_gracefully", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		ws := workspaces.Context{
			CacheDir:       t.TempDir(),
			CacheDirWazero: t.TempDir(),
		}

		require.NotPanics(t, func() { ws.Cleanup(ctx) })
	})
}
