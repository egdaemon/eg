package gitx

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v6"
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

	git := func(t *testing.T, dir string, args ...string) string {
		t.Helper()

		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}

	// base is the base DefaultBase finds in the clone, and the commit it is at.
	base := func(t *testing.T, dir string) (string, string) {
		t.Helper()

		repo, err := gogit.PlainOpen(dir)
		require.NoError(t, err)
		return DefaultBase(repo, "origin")
	}

	// a source where main is the default branch and feature is a branch with its own commit.
	featured := func(t *testing.T) string {
		src := newMainRepo(t, map[string]string{"a.txt": "a"})
		git(t, src, "checkout", "-q", "-b", "feature")
		commitFile(t, src, "f.txt", "f")
		git(t, src, "checkout", "-q", "main")
		commitFile(t, src, "m.txt", "m")
		return src
	}

	t.Run("a branch other than the default carries the default branch as the base", func(t *testing.T) {
		src := featured(t)
		dst := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, Clone(t.Context(), dst, src, "origin", "feature"))
		require.FileExists(t, filepath.Join(dst, "f.txt"))

		name, commit := base(t, dst)
		require.Equal(t, "main", name)
		require.Equal(t, git(t, src, "rev-parse", "main"), commit)
		require.Equal(t, commit, git(t, dst, "rev-parse", "origin/main"))
	})

	t.Run("the base is the branch the remote's HEAD names, not only main or master", func(t *testing.T) {
		src := featured(t)
		git(t, src, "branch", "-M", "develop")
		dst := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, Clone(t.Context(), dst, src, "origin", "feature"))

		name, commit := base(t, dst)
		require.Equal(t, "develop", name)
		require.Equal(t, git(t, src, "rev-parse", "develop"), commit)
	})

	t.Run("cloning the default branch is its own base", func(t *testing.T) {
		src := featured(t)
		dst := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, Clone(t.Context(), dst, src, "origin", "main"))

		name, commit := base(t, dst)
		require.Equal(t, "main", name)
		require.Equal(t, git(t, src, "rev-parse", "main"), commit)
	})

	t.Run("an existing clone gets the base when it is missing", func(t *testing.T) {
		src := featured(t)
		dst := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, Clone(t.Context(), dst, src, "origin", "feature"))

		git(t, dst, "update-ref", "-d", "refs/remotes/origin/main")
		git(t, dst, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
		name, _ := base(t, dst)
		require.Empty(t, name)

		// already up to date is not an error, and still restores the base.
		require.NoError(t, Clone(t.Context(), dst, src, "origin", "feature"))
		name, commit := base(t, dst)
		require.Equal(t, "main", name)
		require.Equal(t, git(t, src, "rev-parse", "main"), commit)
	})

	t.Run("a remote without a default branch still clones", func(t *testing.T) {
		src := featured(t)
		git(t, src, "branch", "-M", "trunk")
		git(t, src, "checkout", "-q", "--detach")
		commitFile(t, src, "d.txt", "d")
		dst := filepath.Join(t.TempDir(), "clone")

		require.NoError(t, Clone(t.Context(), dst, src, "origin", "feature"))
		require.FileExists(t, filepath.Join(dst, "f.txt"))
		name, _ := base(t, dst)
		require.Empty(t, name)
	})

	t.Run("unknown sources and branches error", func(t *testing.T) {
		src := newMainRepo(t, map[string]string{"a.txt": "a"})

		require.Error(t, Clone(t.Context(), filepath.Join(t.TempDir(), "clone"), filepath.Join(t.TempDir(), "missing"), "origin", "main"))
		require.Error(t, Clone(t.Context(), filepath.Join(t.TempDir(), "clone"), src, "origin", "nope"))
	})
}
