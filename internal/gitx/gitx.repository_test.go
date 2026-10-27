package gitx

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// creates a repository with a single commit on main, returns its directory.
func newMainRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	initRepo(t, dir, files)
	require.NoError(t, os.Remove(filepath.Join(dir, "untracked.txt")))

	out, err := exec.Command("git", "-C", dir, "branch", "-M", "main").CombinedOutput()
	require.NoErrorf(t, err, "%s", out)

	return dir
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", "update " + name}} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
	}
}

func TestDetectRoot(t *testing.T) {
	dir := newMainRepo(t, map[string]string{"a.txt": "a"})
	nested := filepath.Join(dir, "nested", "deeper")
	require.NoError(t, os.MkdirAll(nested, 0755))

	t.Chdir(nested)
	root, err := filepath.EvalSymlinks(DetectRoot())
	require.NoError(t, err)
	expected, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	require.Equal(t, expected, root)
}

func TestCommitish(t *testing.T) {
	dir := newMainRepo(t, map[string]string{"a.txt": "a"})

	t.Run("resolves references to commits", func(t *testing.T) {
		out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").CombinedOutput()
		require.NoError(t, err)
		expected := string(out[:len(out)-1])

		for _, treeish := range []string{"HEAD", "main", expected} {
			hash, err := Commitish(dir, treeish)
			require.NoError(t, err, treeish)
			require.Equal(t, expected, hash, treeish)
		}
	})

	t.Run("unresolvable references error", func(t *testing.T) {
		hash, err := Commitish(dir, "nope")
		require.Error(t, err)
		require.Empty(t, hash)
	})

	t.Run("non repositories error", func(t *testing.T) {
		_, err := Commitish(t.TempDir(), "HEAD")
		require.Error(t, err)
	})
}

func TestVCSDownloadToken(t *testing.T) {
	claims := VCSDownloadToken("account", "git@github.com:egdaemon/eg.git")
	require.Equal(t, "git@github.com:egdaemon/eg.git", claims.Subject)
	require.Equal(t, "account", claims.Issuer)
	require.WithinDuration(t, time.Now().Add(24*time.Hour), claims.ExpiresAt.Time, time.Minute)
}

func TestRepositoryUsername(t *testing.T) {
	current, err := user.Current()
	require.NoError(t, err)

	t.Run("owner of the repository", func(t *testing.T) {
		require.Equal(t, current.Username, RepositoryUsername(newMainRepo(t, map[string]string{"a.txt": "a"})))
	})

	t.Run("falls back to the current user when the repository can't be inspected", func(t *testing.T) {
		require.Equal(t, current.Username, RepositoryUsername(filepath.Join(t.TempDir(), "missing")))
	})
}

func TestClone(t *testing.T) {
	t.Run("clones a branch into a new directory", func(t *testing.T) {
		src := newMainRepo(t, map[string]string{"a.txt": "a"})
		dst := filepath.Join(t.TempDir(), "clone")

		require.NoError(t, Clone(t.Context(), dst, src, "origin", "main"))
		require.FileExists(t, filepath.Join(dst, "a.txt"))
	})

	t.Run("an existing clone is fetched and checked out", func(t *testing.T) {
		src := newMainRepo(t, map[string]string{"a.txt": "a"})
		dst := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, Clone(t.Context(), dst, src, "origin", "main"))

		// already up to date is not an error.
		require.NoError(t, Clone(t.Context(), dst, src, "origin", "main"))

		// point the clone's origin at the source so the fetch sees new commits.
		out, err := exec.Command("git", "-C", dst, "remote", "set-url", "origin", src).CombinedOutput()
		require.NoErrorf(t, err, "%s", out)
		commitFile(t, src, "b.txt", "b")

		require.NoError(t, Clone(t.Context(), dst, src, "origin", "main"))
		out, err = exec.Command("git", "-C", dst, "rev-parse", "origin/main").CombinedOutput()
		require.NoErrorf(t, err, "%s", out)
		expected, err := exec.Command("git", "-C", src, "rev-parse", "main").CombinedOutput()
		require.NoError(t, err)
		require.Equal(t, string(expected), string(out))
	})

	t.Run("an existing clone without the remote errors", func(t *testing.T) {
		src := newMainRepo(t, map[string]string{"a.txt": "a"})
		dst := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, Clone(t.Context(), dst, src, "origin", "main"))

		require.Error(t, Clone(t.Context(), dst, src, "missing", "main"))
	})

	t.Run("unknown sources and branches error", func(t *testing.T) {
		src := newMainRepo(t, map[string]string{"a.txt": "a"})

		require.Error(t, Clone(t.Context(), filepath.Join(t.TempDir(), "clone"), filepath.Join(t.TempDir(), "missing"), "origin", "main"))
		require.Error(t, Clone(t.Context(), filepath.Join(t.TempDir(), "clone"), src, "origin", "nope"))
	})
}
