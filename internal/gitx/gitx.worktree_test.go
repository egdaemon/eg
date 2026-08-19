package gitx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorktree(t *testing.T) {
	cloneBehaviorTests(t, Worktree)

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

	t.Run("fails when repo is not a git repository", func(t *testing.T) {
		require.Error(t, Worktree(t.Context(), t.TempDir(), filepath.Join(t.TempDir(), "wt")))
	})
}
