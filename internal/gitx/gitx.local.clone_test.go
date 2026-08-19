package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalClone(t *testing.T) {
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

	t.Run("does not mutate the source repository's own checkout", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir))

		_, err := os.Stat(filepath.Join(repo, "untracked.txt"))
		require.NoError(t, err, "source repository's own working tree should be untouched")
	})

	t.Run("fails when repo is not a git repository", func(t *testing.T) {
		require.Error(t, LocalClone(t.Context(), t.TempDir(), filepath.Join(t.TempDir(), "clone")))
	})

	t.Run("preserves origin SSH URL", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		origin := "git@github.com:user/repo.git"

		cmd := exec.Command("git", "-C", repo, "remote", "set-url", "origin", origin)
		_, err := cmd.CombinedOutput()
		require.NoError(t, err)

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir))

		cloneOrigin, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").CombinedOutput()
		require.NoError(t, err, "clone should have a valid origin URL")
		require.Equal(t, origin, strings.TrimSpace(string(cloneOrigin)))
	})

	t.Run("preserves origin SSH URL via git@github.com", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		origin := "ssh://git@github.com/user/repo.git"

		cmd := exec.Command("git", "-C", repo, "remote", "set-url", "origin", origin)
		_, err := cmd.CombinedOutput()
		require.NoError(t, err)

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir))

		cloneOrigin, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").CombinedOutput()
		require.NoError(t, err, "clone should have a valid origin URL")
		require.Equal(t, origin, strings.TrimSpace(string(cloneOrigin)))
	})
}
