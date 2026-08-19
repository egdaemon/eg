package gitx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorktree(t *testing.T) {
	t.Run("checks out committed state only", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})

		dir := filepath.Join(t.TempDir(), "wt")
		require.NoError(t, Worktree(t.Context(), repo, dir))

		fi, err := os.Lstat(dir)
		require.NoError(t, err)
		require.Zero(t, fi.Mode()&os.ModeSymlink, "worktree dir should not be a symlink")

		_, err = os.Stat(filepath.Join(dir, "main.go"))
		require.NoError(t, err, "committed file should be checked out")

		_, err = os.Stat(filepath.Join(dir, "untracked.txt"))
		require.ErrorIs(t, err, os.ErrNotExist, "uncommitted/untracked files should not carry over")
	})

	t.Run("does not mutate the source repository's own checkout", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})

		dir := filepath.Join(t.TempDir(), "wt")
		require.NoError(t, Worktree(t.Context(), repo, dir))

		// repo's own working copy still has the untracked file initRepo left behind.
		_, err := os.Stat(filepath.Join(repo, "untracked.txt"))
		require.NoError(t, err, "source repository's own working tree should be untouched")
	})

	t.Run("fails when repo is not a git repository", func(t *testing.T) {
		require.Error(t, Worktree(t.Context(), t.TempDir(), filepath.Join(t.TempDir(), "wt")))
	})
}
