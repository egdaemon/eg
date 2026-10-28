package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// CloneFn is the common interface for cloning / checkout strategies.
// Both LocalClone and Worktree satisfy this signature.
type CloneFn func(ctx context.Context, repo, dir string) error

// cloneBehaviorTests runs strategy-agnostic behavior tests against the
// given cloneFn. Call it from TestLocalClone and TestWorktree so each
// strategy is checked against the same invariants.
func cloneBehaviorTests(t *testing.T, cloneFn CloneFn) {
	t.Helper()

	t.Run("does not mutate the source repository's own checkout", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, cloneFn(t.Context(), repo, dir))

		_, err := os.Stat(filepath.Join(repo, "untracked.txt"))
		require.NoError(t, err, "source repository's own working tree should be untouched")
	})

	t.Run("preserves origin SSH URL", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		origin := "git@github.com:user/repo.git"

		cmd := exec.Command("git", "-C", repo, "remote", "set-url", "origin", origin)
		_, err := cmd.CombinedOutput()
		require.NoError(t, err)

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, cloneFn(t.Context(), repo, dir))

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
		require.NoError(t, cloneFn(t.Context(), repo, dir))

		cloneOrigin, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").CombinedOutput()
		require.NoError(t, err, "clone should have a valid origin URL")
		require.Equal(t, origin, strings.TrimSpace(string(cloneOrigin)))
	})

	t.Run("origin must not be munged", func(t *testing.T) {
		// originMustNotBeMunged ensures the origin URL survives any mutations by
		// a strategy intact -- no trailing whitespace, no newline leakage, no character mangling.
		// This is a regression guard for the git remote get-url + strings.TrimSpace
		// path in LocalClone, where the raw stdout from git carries a trailing \n
		// that must not leak into git remote add (which would mangle the origin in
		// downstream volume-name construction).
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		origin := "git@github.com:user/repo.git"

		cmd := exec.Command("git", "-C", repo, "remote", "set-url", "origin", origin)
		_, err := cmd.CombinedOutput()
		require.NoError(t, err)

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, cloneFn(t.Context(), repo, dir))

		// The origin stored in the clone must match exactly -- no trailing
		// whitespace, no embedded newlines, no character substitution.
		cloneOrigin, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").CombinedOutput()
		require.NoError(t, err, "clone should have a valid origin URL")
		cleaned := string(cloneOrigin)
		// git stdout always includes a trailing \n, so trim before comparing.
		// The trimmed result must match the origin exactly — if LocalClone
		// failed to strip a newline from git remote get-url, this fails.
		require.Equal(t, origin+"\n", cleaned, "origin URL must not be munged")
		// Verify trimming a second time is a no-op: nothing was hidden.
		require.Equal(t, cleaned, strings.TrimSpace(cleaned)+"\n", "trimmed origin has no hidden whitespace")
	})

	// regression guard: compute upload resolves the --git-ref (main by default) against the
	// checkout, a clone only has the source's current branch unless the others are carried over.
	t.Run("branches of the source remain resolvable", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		want := gitOut(t, repo, "rev-parse", "main")

		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "feature")
		head := gitOut(t, repo, "rev-parse", "HEAD")
		require.NotEqual(t, want, head)

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, cloneFn(t.Context(), repo, dir))

		got, err := Commitish(dir, "main")
		require.NoError(t, err, "a branch other than the one checked out in the source must resolve in the clone")
		require.Equal(t, want, got)

		got, err = Commitish(dir, "HEAD")
		require.NoError(t, err)
		require.Equal(t, head, got, "HEAD must be the commit checked out in the source")
	})

	t.Run("current branch of the source remains resolvable", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		want := gitOut(t, repo, "rev-parse", "feature")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, cloneFn(t.Context(), repo, dir))

		got, err := Commitish(dir, "feature")
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}

// gitOut runs git within dir returning its trimmed output.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, out)

	return strings.TrimSpace(string(out))
}

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
	run("remote", "add", "origin", "https://example.com/dummy/repo.git")

	for name, content := range committed {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	run("add", ".")
	run("commit", "-q", "-m", "init")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("dirty"), 0644))
}
