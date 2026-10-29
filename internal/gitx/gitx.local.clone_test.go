package gitx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/stretchr/testify/require"
)

func TestLocalClone(t *testing.T) {
	cloneBehaviorTests(t, func(ctx context.Context, repo, dir string) error {
		return LocalClone(ctx, repo, dir, "main", "HEAD")
	})

	t.Run("checks out committed state only", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

		_, err := os.Stat(filepath.Join(dir, "main.go"))
		require.NoError(t, err, "committed file should be checked out")

		_, err = os.Stat(filepath.Join(dir, "untracked.txt"))
		require.ErrorIs(t, err, os.ErrNotExist, "uncommitted/untracked files should not carry over")
	})

	t.Run("checkout has its own independent .git directory", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

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
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

		_, err := os.Stat(filepath.Join(dir, "main.go"))
		require.NoError(t, err, "committed file should be checked out")

		// clone should have its own independent .git directory
		fi, err := os.Stat(filepath.Join(dir, ".git"))
		require.NoError(t, err)
		require.True(t, fi.IsDir(), ".git should be a real directory, not a worktree gitdir file")
	})

	t.Run("carries a base branch that only exists as a remote-tracking ref", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		want := gitOut(t, repo, "rev-parse", "main")
		gitOut(t, repo, "update-ref", "refs/remotes/origin/main", "main")
		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "feature")
		gitOut(t, repo, "branch", "-q", "-D", "main")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

		got, err := Commitish(dir, "main")
		require.NoError(t, err, "the base must resolve even though the source has no local main")
		require.Equal(t, want, got)

		got, err = Commitish(dir, "origin/main")
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("keeps the complete history and all branches", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		for i := range 5 {
			require.NoError(t, os.WriteFile(filepath.Join(repo, fmt.Sprintf("old%d.txt", i)), []byte(strings.Repeat(fmt.Sprintf("%d old\n", i), 500)), 0644))
			gitOut(t, repo, "add", ".")
			gitOut(t, repo, "commit", "-q", "-m", fmt.Sprintf("old %d", i))
		}

		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		for i := range 3 {
			require.NoError(t, os.WriteFile(filepath.Join(repo, fmt.Sprintf("new%d.txt", i)), []byte(fmt.Sprintf("%d new\n", i)), 0644))
			gitOut(t, repo, "add", ".")
			gitOut(t, repo, "commit", "-q", "-m", fmt.Sprintf("feature %d", i))
		}

		gitOut(t, repo, "checkout", "-q", "main")
		for i := range 2 {
			require.NoError(t, os.WriteFile(filepath.Join(repo, fmt.Sprintf("main%d.txt", i)), []byte(fmt.Sprintf("%d main\n", i)), 0644))
			gitOut(t, repo, "add", ".")
			gitOut(t, repo, "commit", "-q", "-m", fmt.Sprintf("main %d", i))
		}
		gitOut(t, repo, "checkout", "-q", "feature")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

		require.Equal(t, gitOut(t, repo, "rev-parse", "HEAD"), gitOut(t, dir, "rev-parse", "HEAD"))
		require.Equal(t, gitOut(t, repo, "rev-parse", "main"), gitOut(t, dir, "rev-parse", "main"))
		require.Equal(t, gitOut(t, repo, "diff", "--name-only", "main..HEAD"), gitOut(t, dir, "diff", "--name-only", "main..HEAD"))
		require.Equal(t, "3", gitOut(t, dir, "rev-list", "--count", "main..HEAD"))
		require.Equal(t, gitOut(t, repo, "rev-list", "--count", "--all"), gitOut(t, dir, "rev-list", "--count", "--all"), "nothing is cut from the history")
		require.Equal(t, gitOut(t, repo, "merge-base", "main", "HEAD"), gitOut(t, dir, "merge-base", "main", "HEAD"))
		require.Equal(t, gitOut(t, repo, "rev-parse", "feature"), gitOut(t, dir, "rev-parse", "feature"), "the branch of the source is still there")

		_, err := os.Stat(filepath.Join(dir, ".git", "shallow"))
		require.ErrorIs(t, err, os.ErrNotExist, "the clone isn't shallow")
	})

	t.Run("a merge of the base into the branch still diffs against the base", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("feature"), 0644))
		gitOut(t, repo, "add", ".")
		gitOut(t, repo, "commit", "-q", "-m", "feature")
		gitOut(t, repo, "checkout", "-q", "main")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "main.txt"), []byte("main"), 0644))
		gitOut(t, repo, "add", ".")
		gitOut(t, repo, "commit", "-q", "-m", "main")
		gitOut(t, repo, "checkout", "-q", "feature")
		gitOut(t, repo, "merge", "-q", "--no-edit", "main")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

		got := gitOut(t, dir, "diff", "--name-only", "main..HEAD")
		require.Contains(t, got, "feature.txt")
		require.NotContains(t, got, "main.txt", "the merged base must not show as a change")
		require.Equal(t, gitOut(t, repo, "diff", "--name-only", "main..HEAD"), got)
		require.Equal(t, gitOut(t, repo, "rev-list", "--count", "main..HEAD"), gitOut(t, dir, "rev-list", "--count", "main..HEAD"), "the commits of the branch are kept, not only the merge")
	})

	t.Run("without a base branch no base is recorded", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "feature")
		gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "second")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

		require.Equal(t, gitOut(t, repo, "rev-parse", "HEAD"), gitOut(t, dir, "rev-parse", "HEAD"))
		require.Empty(t, gitOut(t, dir, "config", "--local", "--default", "", "--get", "init.defaultBranch"))
	})

	t.Run("a clone can be cloned again", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		gitOut(t, repo, "update-ref", "refs/remotes/origin/main", "main")
		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		for i := range 3 {
			require.NoError(t, os.WriteFile(filepath.Join(repo, fmt.Sprintf("new%d.txt", i)), []byte(fmt.Sprintf("%d new\n", i)), 0644))
			gitOut(t, repo, "add", ".")
			gitOut(t, repo, "commit", "-q", "-m", fmt.Sprintf("feature %d", i))
		}
		gitOut(t, repo, "branch", "-q", "-D", "main")

		first := filepath.Join(t.TempDir(), "first")
		require.NoError(t, LocalClone(t.Context(), repo, first, "main", "HEAD"))

		second := filepath.Join(t.TempDir(), "second")
		require.NoError(t, LocalClone(t.Context(), first, second, "main", "HEAD"))

		require.Equal(t, gitOut(t, repo, "rev-parse", "HEAD"), gitOut(t, second, "rev-parse", "HEAD"))
		require.Equal(t, gitOut(t, repo, "rev-parse", "origin/main"), gitOut(t, second, "rev-parse", "main"))
		r, err := git.PlainOpen(second)
		require.NoError(t, err)
		name, commit := DefaultBase(r, "origin")
		require.Equal(t, "main", name, "the base survives cloning a clone")
		require.Equal(t, gitOut(t, repo, "rev-parse", "origin/main"), commit)
		require.Equal(t, "3", gitOut(t, second, "rev-list", "--count", "main..HEAD"))
		require.Equal(t, gitOut(t, repo, "diff", "--name-only", "origin/main..HEAD"), gitOut(t, second, "diff", "--name-only", "main..HEAD"))
	})

	t.Run("the base named by the caller is the one carried", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		gitOut(t, repo, "checkout", "-q", "-b", "develop")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "develop.txt"), []byte("develop"), 0644))
		gitOut(t, repo, "add", ".")
		gitOut(t, repo, "commit", "-q", "-m", "develop")
		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("feature"), 0644))
		gitOut(t, repo, "add", ".")
		gitOut(t, repo, "commit", "-q", "-m", "feature")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "develop", "HEAD"))

		require.Equal(t, gitOut(t, repo, "rev-parse", "develop"), gitOut(t, dir, "rev-parse", "develop"))
		require.Equal(t, "feature.txt", gitOut(t, dir, "diff", "--name-only", "develop..HEAD"))
		require.Equal(t, "1", gitOut(t, dir, "rev-list", "--count", "develop..HEAD"))
		r, err := git.PlainOpen(dir)
		require.NoError(t, err)
		name, commit := DefaultBase(r, "origin")
		require.Equal(t, "develop", name, "the clone records its base so it can be found again")
		require.Equal(t, gitOut(t, repo, "rev-parse", "develop"), commit)
		require.Equal(t, "develop", gitOut(t, dir, "config", "--local", "--get", "init.defaultBranch"), "recorded as the default branch of the clone")
		require.Empty(t, gitOut(t, dir, "for-each-ref", "refs/remotes"), "the source never had a remote-tracking develop so the clone doesn't invent one")
	})

	t.Run("a base only on the remote keeps its remote-tracking branch", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		gitOut(t, repo, "update-ref", "refs/remotes/origin/main", "main")
		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "feature")
		gitOut(t, repo, "branch", "-q", "-D", "main")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

		r, err := git.PlainOpen(dir)
		require.NoError(t, err)
		name, commit := DefaultBase(r, "origin")
		require.Equal(t, "main", name)
		require.Equal(t, gitOut(t, repo, "rev-parse", "origin/main"), commit)
		require.Equal(t, gitOut(t, repo, "rev-parse", "origin/main"), gitOut(t, dir, "rev-parse", "origin/main"))
		require.Equal(t, "main", gitOut(t, dir, "config", "--local", "--get", "init.defaultBranch"))
	})

	t.Run("a base given as a commit id bounds the range", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		for i := range 2 {
			gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", fmt.Sprintf("old %d", i))
		}
		base := gitOut(t, repo, "rev-parse", "HEAD")
		for i := range 3 {
			require.NoError(t, os.WriteFile(filepath.Join(repo, fmt.Sprintf("new%d.txt", i)), []byte("new"), 0644))
			gitOut(t, repo, "add", ".")
			gitOut(t, repo, "commit", "-q", "-m", fmt.Sprintf("new %d", i))
		}

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, base, "HEAD"))

		require.Equal(t, "3", gitOut(t, dir, "rev-list", "--count", base+"..HEAD"))
		require.Equal(t, gitOut(t, repo, "diff", "--name-only", base+"..HEAD"), gitOut(t, dir, "diff", "--name-only", base+"..HEAD"))
	})

	t.Run("a base that does not resolve is tolerated", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "second")

		for _, base := range []string{"", "nope", "-nope"} {
			dir := filepath.Join(t.TempDir(), "clone")
			require.NoError(t, LocalClone(t.Context(), repo, dir, base, "HEAD"), base)
			require.Equal(t, gitOut(t, repo, "rev-parse", "HEAD"), gitOut(t, dir, "rev-parse", "HEAD"), base)
			require.Empty(t, gitOut(t, dir, "config", "--local", "--default", "", "--get", "init.defaultBranch"), base)
		}
	})

	t.Run("the head named by the caller is what is checked out", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("feature"), 0644))
		gitOut(t, repo, "add", ".")
		gitOut(t, repo, "commit", "-q", "-m", "feature")
		want := gitOut(t, repo, "rev-parse", "feature")
		gitOut(t, repo, "checkout", "-q", "main")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "feature"))

		require.Equal(t, want, gitOut(t, dir, "rev-parse", "HEAD"))
		_, err := os.Stat(filepath.Join(dir, "feature.txt"))
		require.NoError(t, err)
	})

	t.Run("a head that does not resolve fails", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})

		for _, head := range []string{"nope", "-nope"} {
			require.Error(t, LocalClone(t.Context(), repo, filepath.Join(t.TempDir(), "clone"), "main", head), head)
		}
	})

	t.Run("tags come along", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		gitOut(t, repo, "tag", "-a", "-m", "old release", "v-old")
		gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "base")
		gitOut(t, repo, "tag", "-a", "-m", "release", "v-base")
		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "first")
		gitOut(t, repo, "tag", "-a", "-m", "candidate", "rc-first")
		gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "second")
		gitOut(t, repo, "tag", "v-head")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

		require.ElementsMatch(t, []string{"v-old", "v-base", "rc-first", "v-head"}, strings.Fields(gitOut(t, dir, "tag")))
		require.Equal(t, gitOut(t, repo, "rev-parse", "rc-first^{commit}"), gitOut(t, dir, "rev-parse", "rc-first^{commit}"))
	})

	t.Run("copies the objects when they can't be hardlinked", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		gitOut(t, repo, "commit-graph", "write", "--reachable")

		other, err := os.MkdirTemp("/dev/shm", "eg-localclone-*")
		if err != nil {
			t.Skip("no second filesystem to clone onto", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(other) })

		var source, target syscall.Stat_t
		require.NoError(t, syscall.Stat(repo, &source))
		require.NoError(t, syscall.Stat(other, &target))
		if source.Dev == target.Dev {
			t.Skip("the filesystem is the same, hardlinks work")
		}

		dir := filepath.Join(other, "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))
		require.Equal(t, gitOut(t, repo, "rev-parse", "HEAD"), gitOut(t, dir, "rev-parse", "HEAD"))
		require.Equal(t, "main", gitOut(t, dir, "config", "--local", "--get", "init.defaultBranch"))
	})

	t.Run("the refs of the source are the refs of the clone", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"main.go": "package main"})
		gitOut(t, repo, "branch", "-M", "main")
		gitOut(t, repo, "tag", "v1")
		gitOut(t, repo, "branch", "develop")
		gitOut(t, repo, "update-ref", "refs/remotes/origin/main", "main")
		gitOut(t, repo, "update-ref", "refs/remotes/origin/release", "develop")
		gitOut(t, repo, "checkout", "-q", "-b", "feature")
		gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "feature")

		dir := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, dir, "main", "HEAD"))

		refs := func(path string) string {
			return gitOut(t, path, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads", "refs/remotes/origin", "refs/tags")
		}
		require.Equal(t, refs(repo), refs(dir), "no relabelled refs, none missing, none invented")
	})

	t.Run("fails when repo is not a git repository", func(t *testing.T) {
		require.Error(t, LocalClone(t.Context(), t.TempDir(), filepath.Join(t.TempDir(), "clone"), "main", "HEAD"))
	})
}
