package gitx

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAutoPatchSession(t *testing.T) {
	t.Run("commits to the session ref and creates the branch from it", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))

		require.NoError(t, AutoPatch(io.Discard).From(clone).To(repo).Session("abc").Branch("eg/named").Apply(t.Context()))

		ref, err := exec.Command("git", "-C", repo, "rev-parse", SessionRef("abc")).CombinedOutput()
		require.NoError(t, err, string(ref))
		branch, err := exec.Command("git", "-C", repo, "rev-parse", "refs/heads/eg/named").CombinedOutput()
		require.NoError(t, err, string(branch))
		require.Equal(t, string(ref), string(branch))
		show, err := exec.Command("git", "-C", repo, "show", SessionRef("abc")+":a.txt").CombinedOutput()
		require.NoError(t, err, string(show))
		require.Equal(t, "two\n", string(show))
	})

	t.Run("without auto branch only the ref is written, and the branch can be renamed in between", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))

		require.NoError(t, AutoPatch(io.Discard).From(clone).To(repo).Session("abc").Branch("eg/named").AutoBranch(false).Apply(t.Context()))

		require.NoError(t, exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", SessionRef("abc")).Run())
		require.Error(t, exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", "refs/heads/eg/named").Run())
	})

	t.Run("extends the ref on a second run and fast forwards the branch", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n", "b.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))

		for _, name := range []string{"a.txt", "b.txt"} {
			require.NoError(t, os.WriteFile(filepath.Join(clone, name), []byte("two\n"), 0644))
			require.NoError(t, AutoPatch(io.Discard).From(clone).To(repo).Session("abc").Branch("eg/named").Apply(t.Context()))
			raw, err := exec.Command("git", "-C", clone, "checkout", "-q", "--", ".").CombinedOutput()
			require.NoError(t, err, string(raw))
		}

		count, err := exec.Command("git", "-C", repo, "rev-list", "--count", "main.."+SessionRef("abc")).CombinedOutput()
		require.NoError(t, err, string(count))
		require.Equal(t, "2", strings.TrimSpace(string(count)))
		branch, err := exec.Command("git", "-C", repo, "rev-parse", "refs/heads/eg/named").CombinedOutput()
		require.NoError(t, err, string(branch))
		ref, err := exec.Command("git", "-C", repo, "rev-parse", SessionRef("abc")).CombinedOutput()
		require.NoError(t, err, string(ref))
		require.Equal(t, string(ref), string(branch))
	})

	t.Run("leaves a branch that was changed by something else alone and keeps the ref", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n", "b.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))

		// the branch has a commit the session does not.
		for _, args := range [][]string{{"checkout", "-q", "-b", "eg/named"}, {"commit", "-q", "--allow-empty", "-m", "elsewhere"}, {"checkout", "-q", "main"}} {
			raw, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
			require.NoError(t, err, string(raw))
		}
		before, err := exec.Command("git", "-C", repo, "rev-parse", "refs/heads/eg/named").CombinedOutput()
		require.NoError(t, err, string(before))

		require.Error(t, AutoPatch(io.Discard).From(clone).To(repo).Session("abc").Branch("eg/named").Apply(t.Context()))

		after, err := exec.Command("git", "-C", repo, "rev-parse", "refs/heads/eg/named").CombinedOutput()
		require.NoError(t, err, string(after))
		require.Equal(t, string(before), string(after))
		require.NoError(t, exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", SessionRef("abc")).Run())
	})

	t.Run("carries the commits made after the clone was rebased onto the latest", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n", "b.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))

		run := func(dir string, args ...string) string {
			raw, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
			require.NoError(t, err, string(raw))
			return strings.TrimSpace(string(raw))
		}

		// a commit in the clone, and one by the user that the clone is then rebased onto.
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("clone\n"), 0644))
		run(clone, "commit", "-q", "-am", "clone work")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "b.txt"), []byte("user\n"), 0644))
		run(repo, "commit", "-q", "-am", "user work")
		run(clone, "fetch", "-q", "file://"+repo, "+HEAD:"+RefLatest)
		run(clone, "rebase", RefLatest)

		require.NoError(t, AutoPatch(io.Discard).From(clone).To(repo).Session("abc").Branch("eg/named").Apply(t.Context()))

		// only the clone's commit is the session's: the user's own commit is the base, not carried.
		require.Equal(t, "1", run(repo, "rev-list", "--count", "main.."+SessionRef("abc")))
		require.Equal(t, "clone work", run(repo, "log", "--format=%s", "-1", SessionRef("abc")))
	})
}
