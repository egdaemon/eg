package gitx

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/compute"
	"github.com/stretchr/testify/require"
)

// initRepo creates a git repository at dir with a single commit containing
// the given committed files, then leaves an additional uncommitted/untracked
// file behind so tests can assert worktree checkouts only see committed
// state.
func initRepo(t *testing.T, dir string, committed map[string]string) {
	t.Helper()

	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
	}

	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")

	for name, content := range committed {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	run("add", ".")
	run("commit", "-q", "-m", "init")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("dirty"), 0644))
}

func TestIsRepository(t *testing.T) {
	t.Run("true for a git repository", func(t *testing.T) {
		dir := t.TempDir()
		initRepo(t, dir, map[string]string{"main.go": "package main"})
		require.True(t, IsRepository(dir))
	})

	t.Run("false for a plain directory", func(t *testing.T) {
		require.False(t, IsRepository(t.TempDir()))
	})
}

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
}

func TestBearer(t *testing.T) {
	t.Run("returns password from token file", func(t *testing.T) {
		dir := t.TempDir()
		writeTokenFile(t, dir, "filetoken")
		require.Equal(t, "filetoken", Bearer(dir))
	})

	t.Run("empty when file absent", func(t *testing.T) {
		require.Equal(t, "", Bearer(t.TempDir()))
	})
}

func writeTokenFile(t *testing.T, dir, password string) {
	t.Helper()
	encoded, err := json.Marshal(&compute.GitCredentialsHTTP{Password: password})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vcsaccess.token"), encoded, 0600))
}
