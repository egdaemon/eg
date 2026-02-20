package fsx_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestKeepNewestN(t *testing.T) {
	t.Run("keeps_n_newest_yields_rest", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		touch(t, filepath.Join(root, "a", ".keep"), 3*time.Hour)
		touch(t, filepath.Join(root, "b", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(root, "c", ".keep"), 1*time.Hour)

		// keep 1 newest (c), yield the rest (a, b)
		require.Equal(t, []string{
			filepath.Join(root, "a"),
			filepath.Join(root, "b"),
		}, collected(t, ctx, fsx.KeepNewestN(1, fsx.Find(root))))
	})

	t.Run("keep_zero_yields_all", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		touch(t, filepath.Join(root, "a", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(root, "b", ".keep"), 1*time.Hour)

		require.Equal(t, []string{
			filepath.Join(root, "a"),
			filepath.Join(root, "b"),
		}, collected(t, ctx, fsx.KeepNewestN(0, fsx.Find(root))))
	})

	t.Run("keep_n_gte_count_yields_nothing", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		touch(t, filepath.Join(root, "a", ".keep"), 2*time.Hour)
		touch(t, filepath.Join(root, "b", ".keep"), 1*time.Hour)

		require.Empty(t, collected(t, ctx, fsx.KeepNewestN(5, fsx.Find(root))))
	})

	t.Run("empty_input_yields_nothing", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		require.Empty(t, collected(t, ctx, fsx.KeepNewestN(3, fsx.Find(t.TempDir()))))
	})

	t.Run("directory_with_only_files_returns_all_but_newest_n", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		root := t.TempDir()
		touch(t, filepath.Join(root, "old.txt"), 3*time.Hour)
		touch(t, filepath.Join(root, "mid.txt"), 2*time.Hour)
		touch(t, filepath.Join(root, "new.txt"), 1*time.Hour)

		// keep 1 newest (new.txt), yield the rest
		require.Equal(t, []string{
			filepath.Join(root, "mid.txt"),
			filepath.Join(root, "old.txt"),
		}, collected(t, ctx, fsx.KeepNewestN(1, fsx.Find(root))))
	})

	t.Run("propagates_upstream_error", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		expected := errors.New("upstream error")
		s := fsx.KeepNewestN(1, iterx.Error[string](expected))
		for range s.Each(ctx) {}
		require.ErrorIs(t, s.Err(), expected)
	})
}
