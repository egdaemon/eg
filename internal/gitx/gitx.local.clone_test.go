package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalClone(t *testing.T) {
	cloneBehaviorTests(t, LocalClone)

	t.Run("checks out committed state only", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir))

		_, err := os.Stat(filepath.Join(dir, "main.go"))
		require.NoError(t, err, "committed file should be checked out")

		_, err = os.Stat(filepath.Join(dir, "untracked.txt"))
		require.ErrorIs(t, err, os.ErrNotExist, "uncommitted/untracked files should not carry over")
	})

	t.Run("checkout has its own independent .git directory", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir))

		fi, err := os.Stat(filepath.Join(dir, ".git"))
		require.NoError(t, err)
		require.True(t, fi.IsDir(), ".git should be a real directory, not a worktree gitdir file")
	})

	t.Run("succeeds when repo has no origin remote", func(t *testing.T) {
		repo := t.TempDir()

		run := func(args ...string) {
			cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
			out, err := cmd.CombinedOutput()
			require.NoErrorf(t, err, "git %v: %s", args, out)
		}

		run("init", "-q")
		run("config", "user.email", "test@example.com")
		run("config", "user.name", "test")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main"), 0644))
		run("add", ".")
		run("commit", "-q", "-m", "init")
		// intentionally skip adding an origin remote

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir))

		_, err := os.Stat(filepath.Join(dir, "main.go"))
		require.NoError(t, err, "committed file should be checked out")

		// clone should have its own independent .git directory
		fi, err := os.Stat(filepath.Join(dir, ".git"))
		require.NoError(t, err)
		require.True(t, fi.IsDir(), ".git should be a real directory, not a worktree gitdir file")
	})

	t.Run("fails when repo is not a git repository", func(t *testing.T) {
		require.Error(t, LocalClone(t.Context(), t.TempDir(), filepath.Join(t.TempDir(), "clone")))
	})
}
