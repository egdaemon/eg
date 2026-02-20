package fsx_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func touch(t *testing.T, path string, age time.Duration) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte{}, 0644))
	ts := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(path, ts, ts))
}

func collected(t *testing.T, ctx context.Context, s iterx.Seq[string]) []string {
	t.Helper()
	var results []string
	for v := range s.Each(ctx) {
		results = append(results, v)
	}
	require.NoError(t, s.Err())
	slices.Sort(results)
	return results
}

func TestFind(t *testing.T) {
	t.Run("no_options_returns_all_subdirs", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		touch(t, filepath.Join(root, "a", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(root, "b", ".keep"), 30*time.Minute)
		touch(t, filepath.Join(root, "c", ".keep"), 10*time.Minute)

		require.Equal(t, []string{
			filepath.Join(root, "a"),
			filepath.Join(root, "b"),
			filepath.Join(root, "c"),
		}, collected(t, ctx, fsx.Find(root)))
	})

	t.Run("maxage_includes_old_excludes_recent", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		touch(t, filepath.Join(root, "old", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(root, "recent", ".keep"), 30*time.Minute)

		require.Equal(t, []string{
			filepath.Join(root, "old"),
		}, collected(t, ctx, fsx.Find(root, fsx.MaxAge(1*time.Hour))))
	})

	t.Run("maxage_matches_dir_with_any_old_content", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		// "a" has an old file alongside a recent one — oldest content qualifies
		touch(t, filepath.Join(root, "a", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(root, "a", "new.txt"), 5*time.Minute)
		// "b" has only recent content — should not match
		touch(t, filepath.Join(root, "b", ".keep"), 30*time.Minute)

		require.Equal(t, []string{
			filepath.Join(root, "a"),
		}, collected(t, ctx, fsx.Find(root, fsx.MaxAge(1*time.Hour))))
	})

	t.Run("levels_limits_depth_of_scan", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		// recent file at depth 1; old file at depth 2
		touch(t, filepath.Join(root, "a", ".keep"), 30*time.Minute)
		touch(t, filepath.Join(root, "a", "sub", "old.txt"), 2*time.Hour)

		// without Levels: old deep file is seen — matches
		require.Equal(t, []string{
			filepath.Join(root, "a"),
		}, collected(t, ctx, fsx.Find(root, fsx.MaxAge(1*time.Hour))))

		// with Levels(1): deep old.txt not scanned, only recent .keep seen — no match
		require.Empty(t, collected(t, ctx, fsx.Find(root, fsx.MaxAge(1*time.Hour), fsx.Levels(1))))
	})

	t.Run("empty_root_returns_nothing", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		require.Empty(t, collected(t, ctx, fsx.Find(t.TempDir())))
	})

	t.Run("context_cancellation_stops_iteration", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		for _, name := range []string{"a", "b", "c", "d", "e"} {
			touch(t, filepath.Join(root, name, ".keep"), 2*time.Hour)
		}

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		require.Empty(t, collected(t, cancelCtx, fsx.Find(root)))
	})
}
