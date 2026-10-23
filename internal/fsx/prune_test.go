package fsx_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestPrune(t *testing.T) {
	t.Run("removes_all_yielded_paths", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		touch(t, filepath.Join(root, "a", "nested", ".keep"), 3*time.Hour)
		touch(t, filepath.Join(root, "b", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(root, "c", ".keep"), 1*time.Hour)

		// keep the newest (c), prune the rest (a, b) along with their contents.
		require.NoError(t, fsx.Prune(ctx, fsx.KeepNewestN(1, fsx.Find(root))))
		require.NoDirExists(t, filepath.Join(root, "a"))
		require.NoDirExists(t, filepath.Join(root, "b"))
		require.DirExists(t, filepath.Join(root, "c"))
	})

	t.Run("removes_read_only_trees", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		touch(t, filepath.Join(root, "a", "nested", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(root, "b", ".keep"), 1*time.Hour)
		require.NoError(t, os.Chmod(filepath.Join(root, "a", "nested"), 0500))
		require.NoError(t, os.Chmod(filepath.Join(root, "a"), 0500))

		require.NoError(t, fsx.Prune(ctx, fsx.KeepNewestN(1, fsx.Find(root))))
		require.NoDirExists(t, filepath.Join(root, "a"))
		require.DirExists(t, filepath.Join(root, "b"))
	})

	t.Run("nothing_yielded_removes_nothing", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		touch(t, filepath.Join(root, "a", ".keep"), time.Hour)

		require.NoError(t, fsx.Prune(ctx, fsx.KeepNewestN(5, fsx.Find(root))))
		require.DirExists(t, filepath.Join(root, "a"))
	})

	t.Run("returns_the_error_of_the_sequence", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		cause := errors.New("boom")
		require.ErrorIs(t, fsx.Prune(ctx, iterx.Error[string](cause)), cause)
	})

	t.Run("missing_paths_are_not_an_error", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		missing := filepath.Join(t.TempDir(), "missing")
		require.NoError(t, fsx.Prune(ctx, iterx.New(func(_ context.Context, yield func(string) bool) error {
			yield(missing)
			return nil
		})))
		_, err := os.Stat(missing)
		require.True(t, os.IsNotExist(err))
	})
}
