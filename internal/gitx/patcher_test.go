package gitx

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatcher(t *testing.T) {
	// a repository with a.txt and b.txt committed, and a clone of it.
	setup := func(t *testing.T) (repo string, clone string) {
		repo = t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n", "b.txt": "one\n"})

		clone = filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))

		return repo, clone
	}

	read := func(t *testing.T, path string) string {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(raw)
	}

	t.Run("does nothing without a patch", func(t *testing.T) {
		repo, _ := setup(t)
		require.NoError(t, Patch(repo).Apply(t.Context(), nil))
		require.Equal(t, "one\n", read(t, filepath.Join(repo, "a.txt")))
	})

	t.Run("applies modified and new files", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, Patch(repo).Apply(t.Context(), patch))
		require.Equal(t, "two\n", read(t, filepath.Join(repo, "a.txt")))
		require.Equal(t, "new\n", read(t, filepath.Join(repo, "new.txt")))
	})

	t.Run("keeps unrelated uncommitted changes", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(repo, "b.txt"), []byte("mine\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, Patch(repo).Apply(t.Context(), patch))
		require.Equal(t, "two\n", read(t, filepath.Join(repo, "a.txt")))
		require.Equal(t, "mine\n", read(t, filepath.Join(repo, "b.txt")))
	})

	t.Run("skips changes that are already present", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, Patch(repo).Apply(t.Context(), patch))
		require.NoError(t, Patch(repo).Apply(t.Context(), patch))
		require.Equal(t, "two\n", read(t, filepath.Join(repo, "a.txt")))
	})

	t.Run("returns the patch and leaves the repository alone when it does not apply", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("mine\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		err = Patch(repo).Apply(t.Context(), patch)
		var perr *PatchError
		require.ErrorAs(t, err, &perr)
		require.Equal(t, patch, perr.Patch)
		require.Equal(t, "mine\n", read(t, filepath.Join(repo, "a.txt")))

		var out strings.Builder
		require.Equal(t, err, PatchWrite(err, &out))
		require.Equal(t, string(patch), out.String())
	})

	t.Run("patchwrite passes nil and other errors through", func(t *testing.T) {
		var out strings.Builder
		require.NoError(t, PatchWrite(nil, &out))
		other := errors.New("other")
		require.Equal(t, other, PatchWrite(other, &out))
		require.Empty(t, out.String())
	})

	t.Run("does not commit without a branch", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, Patch(repo).Apply(t.Context(), patch))
		out, err := exec.Command("git", "-C", repo, "log", "--format=%s").CombinedOutput()
		require.NoError(t, err)
		require.Equal(t, "init\n", string(out))
	})

	t.Run("commits to the branch regardless of the checked out branch", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, Patch(repo).Branch("workload").Apply(t.Context(), patch))
		head, err := exec.Command("git", "-C", repo, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
		require.NoError(t, err)
		require.Equal(t, "workload\n", string(head))
		log, err := exec.Command("git", "-C", repo, "log", "--format=%s", "workload").CombinedOutput()
		require.NoError(t, err)
		require.Equal(t, "eg: apply workload changes\ninit\n", string(log))
		require.Equal(t, "two\n", read(t, filepath.Join(repo, "a.txt")))
		require.Equal(t, "new\n", read(t, filepath.Join(repo, "new.txt")))

		// the untracked file that was not part of the patch is not committed.
		files, err := exec.Command("git", "-C", repo, "show", "--name-only", "--format=", "workload").CombinedOutput()
		require.NoError(t, err)
		require.NotContains(t, string(files), "untracked.txt")
	})

	t.Run("uses an existing branch", func(t *testing.T) {
		repo, clone := setup(t)
		out, err := exec.Command("git", "-C", repo, "branch", "existing").CombinedOutput()
		require.NoErrorf(t, err, "%s", out)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, Patch(repo).Branch("existing").Apply(t.Context(), patch))
		log, err := exec.Command("git", "-C", repo, "log", "--format=%s", "existing").CombinedOutput()
		require.NoError(t, err)
		require.Equal(t, "eg: apply workload changes\ninit\n", string(log))
	})

	t.Run("does not commit changes that are already present", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, Patch(repo).Branch("workload").Apply(t.Context(), patch))
		require.NoError(t, Patch(repo).Branch("workload").Apply(t.Context(), patch))
		log, err := exec.Command("git", "-C", repo, "log", "--format=%s", "workload").CombinedOutput()
		require.NoError(t, err)
		require.Equal(t, "eg: apply workload changes\ninit\n", string(log))
	})
}

func TestAutoPatch(t *testing.T) {
	t.Run("commits the changes to a new branch without touching the worktree", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n", "b.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "b.txt"), []byte("mine\n"), 0644))

		var out strings.Builder
		require.NoError(t, AutoPatch(&out).From(clone).To(repo).Branch("eg/my-workload-run1").Apply(t.Context()))
		require.Empty(t, out.String())

		show := func(args ...string) string {
			raw, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
			require.NoError(t, err, string(raw))
			return strings.TrimSpace(string(raw))
		}

		require.Equal(t, "two\n", show("show", "eg/my-workload-run1:a.txt")+"\n")
		require.Equal(t, "new\n", show("show", "eg/my-workload-run1:new.txt")+"\n")
		require.Equal(t, "main", show("rev-parse", "--abbrev-ref", "HEAD"))
		require.Equal(t, "M b.txt", show("status", "--porcelain", "--untracked-files=no"))
		a, err := os.ReadFile(filepath.Join(repo, "a.txt"))
		require.NoError(t, err)
		require.Equal(t, "one\n", string(a))
	})

	t.Run("explicit branch is extended", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n", "b.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))

		for _, name := range []string{"a.txt", "b.txt"} {
			require.NoError(t, os.WriteFile(filepath.Join(clone, name), []byte("two\n"), 0644))
			require.NoError(t, AutoPatch(io.Discard).From(clone).To(repo).Branch("eg/custom").Apply(t.Context()))
			raw, err := exec.Command("git", "-C", clone, "checkout", "-q", "--", ".").CombinedOutput()
			require.NoError(t, err, string(raw))
		}

		raw, err := exec.Command("git", "-C", repo, "rev-list", "--count", "main..eg/custom").CombinedOutput()
		require.NoError(t, err, string(raw))
		require.Equal(t, "2", strings.TrimSpace(string(raw)))
	})

	t.Run("uses the first branch name that is not blank", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))

		require.NoError(t, AutoPatch(io.Discard).From(clone).To(repo).Branch("", "  ", "eg/second", "eg/third").Apply(t.Context()))

		raw, err := exec.Command("git", "-C", repo, "branch", "--list", "eg/*").CombinedOutput()
		require.NoError(t, err, string(raw))
		require.Equal(t, "eg/second", strings.TrimSpace(string(raw)))
	})

	t.Run("no changes creates no branch, dry run only prints", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))

		require.NoError(t, AutoPatch(io.Discard).From(clone).To(repo).Branch("eg/none").Apply(t.Context()))

		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		var out strings.Builder
		require.NoError(t, AutoPatch(&out).From(clone).To(repo).Branch("eg/none").DryRun(true).Apply(t.Context()))
		require.Contains(t, out.String(), "+two")

		raw, err := exec.Command("git", "-C", repo, "branch", "--list", "eg/*").CombinedOutput()
		require.NoError(t, err, string(raw))
		require.Empty(t, strings.TrimSpace(string(raw)))
	})

	t.Run("unappliable changes write the patch", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))

		// the repository moved on after the clone, so the patch no longer applies to head.
		require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("other\n"), 0644))
		for _, args := range [][]string{{"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "move"}} {
			raw, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
			require.NoError(t, err, string(raw))
		}

		var out strings.Builder
		err := AutoPatch(&out).From(clone).To(repo).Branch("eg/fail").Apply(t.Context())
		var perr *PatchError
		require.True(t, errors.As(err, &perr))
		require.Contains(t, out.String(), "+two")
	})
}
