package eggit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err, args)
	return strings.TrimSpace(string(out))
}

func submitbranchfixture(t *testing.T) (dir, remote string) {
	t.Helper()
	t.Setenv("GH_TOKEN", "sekret")
	// the tests may themselves run in a workload where cloning is disabled, which switches to writing patches.
	t.Setenv(_eg.EnvUnsafeGitCloneEnabled, "true")
	remote = t.TempDir()
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = remote
	require.NoError(t, cmd.Run())

	dir = t.TempDir()
	gitInit(t, dir)
	gitOutput(t, dir, "remote", "add", "origin", remote)
	t.Chdir(dir)
	return dir, remote
}

func TestSubmitBranch(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		for name, tc := range map[string]struct{ branch, message string }{
			"empty branch":   {"", "msg"},
			"blank branch":   {"  ", "msg"},
			"empty message":  {"feature", ""},
			"invalid branch": {"bad..name", "msg"},
		} {
			t.Run(name, func(t *testing.T) {
				_, _ = submitbranchfixture(t)
				require.Error(t, submitbranch(shell.NewLocal(), tc.branch, tc.message)(t.Context(), nil))
			})
		}
	})

	t.Run("pushes changes to the new branch", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0600))

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "add new file\n\nbody")(t.Context(), nil))

		require.Equal(t, "add new file", gitOutput(t, remote, "log", "-1", "--format=%s", "feature/x"))
		require.Equal(t, "new.txt", gitOutput(t, remote, "ls-tree", "--name-only", "feature/x"))
		require.Equal(t, "feature/x", gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
		require.NotContains(t, gitOutput(t, dir, "config", "--local", "--list"), "sekret")
	})

	t.Run("clean workspace is a noop", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "msg")(t.Context(), nil))

		require.Empty(t, gitOutput(t, remote, "branch", "--list"))
		require.NotEqual(t, "feature/x", gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	})

	t.Run("commits as the head commit author", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)
		t.Setenv(_eg.EnvGitHeadCommitAuthor, "Jane Doe")
		t.Setenv(_eg.EnvGitHeadCommitEmail, "jane@example.com")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0600))

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "msg")(t.Context(), nil))

		require.Equal(t, "Jane Doe <jane@example.com>", gitOutput(t, remote, "log", "-1", "--format=%an <%ae>", "feature/x"))
		require.Equal(t, "Jane Doe <jane@example.com>", gitOutput(t, remote, "log", "-1", "--format=%cn <%ce>", "feature/x"))
	})

	t.Run("commits as egdaemon without a head commit author", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)
		t.Setenv(_eg.EnvGitHeadCommitAuthor, "")
		t.Setenv(_eg.EnvGitHeadCommitEmail, "")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0600))

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "msg")(t.Context(), nil))

		require.Equal(t, "egdaemon <noreply@egdaemon.com>", gitOutput(t, remote, "log", "-1", "--format=%an <%ae>", "feature/x"))
	})

	t.Run("falls back per field", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)
		t.Setenv(_eg.EnvGitHeadCommitAuthor, "Jane Doe")
		t.Setenv(_eg.EnvGitHeadCommitEmail, "")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0600))

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "msg")(t.Context(), nil))

		require.Equal(t, "Jane Doe <noreply@egdaemon.com>", gitOutput(t, remote, "log", "-1", "--format=%an <%ae>", "feature/x"))
	})

	t.Run("pushes to the https form of ssh remotes when there is a token", func(t *testing.T) {
		for remote, expected := range map[string]string{
			"git@github.com:egdaemon/eg.git":          "https://github.com/egdaemon/eg.git",
			"ssh://git@github.com/egdaemon/eg.git":    "https://github.com/egdaemon/eg.git",
			"https://github.com/egdaemon/eg.git":      "https://github.com/egdaemon/eg.git",
			"http://localhost:3000/egdaemon/eg.git":   "http://localhost:3000/egdaemon/eg.git",
			"/srv/git/eg.git":                         "/srv/git/eg.git",
			"git@gitlab.example.com:group/sub/eg.git": "https://gitlab.example.com/group/sub/eg.git",
		} {
			script := "git() { echo '" + remote + "'; }\n" + httpsremote("origin") + "\necho \"$target\""
			cmd := exec.Command("bash", "-c", script)
			out, err := cmd.Output()
			require.NoError(t, err, remote)
			require.Equal(t, expected, strings.TrimSpace(string(out)), remote)
		}
	})

	t.Run("writes a patch instead of a branch when cloning is disabled", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)
		cache := t.TempDir()
		t.Setenv(_eg.EnvUnsafeGitCloneEnabled, "false")
		t.Setenv(_eg.EnvComputeCacheDirectory, cache)

		original := gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("one\n"), 0600))
		gitOutput(t, dir, "add", "tracked.txt")
		gitOutput(t, dir, "commit", "-m", "tracked")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("two\n"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello\n"), 0600))

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "msg")(t.Context(), nil))

		patch, err := os.ReadFile(filepath.Join(cache, "egpatches", "feature-x.patch"))
		require.NoError(t, err)
		require.Contains(t, string(patch), "+two")
		require.Contains(t, string(patch), "b/new.txt")
		require.Empty(t, gitOutput(t, remote, "branch", "--list"))
		require.Equal(t, original, gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
		require.Equal(t, "tracked", gitOutput(t, dir, "log", "-1", "--format=%s"))

		// the patch applies to a pristine clone, which is how the runner hands the changes back.
		clone := filepath.Join(t.TempDir(), "clone")
		gitOutput(t, dir, "clone", dir, clone)
		gitOutput(t, clone, "apply", "--check", filepath.Join(cache, "egpatches", "feature-x.patch"))
	})

	t.Run("writes no patch without changes when cloning is disabled", func(t *testing.T) {
		submitbranchfixture(t)
		cache := t.TempDir()
		t.Setenv(_eg.EnvUnsafeGitCloneEnabled, "false")
		t.Setenv(_eg.EnvComputeCacheDirectory, cache)

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "msg")(t.Context(), nil))

		require.NoDirExists(t, filepath.Join(cache, "egpatches"))
	})

	t.Run("remote option", func(t *testing.T) {
		dir, _ := submitbranchfixture(t)
		other := t.TempDir()
		cmd := exec.Command("git", "init", "--bare")
		cmd.Dir = other
		require.NoError(t, cmd.Run())
		gitOutput(t, dir, "remote", "add", "other", other)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0600))

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "msg", SubmitBranchRemote("other"))(t.Context(), nil))

		require.Equal(t, "feature/x", gitOutput(t, other, "branch", "--list", "--format=%(refname:short)"))
	})
}

func TestSubmitBranchEdgeCases(t *testing.T) {
	t.Run("base option branches from the ref", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)
		initial := gitRevParse(t, dir, "HEAD")
		gitCommitFiles(t, dir, []string{"later.txt"}, "later commit")
		// an untracked file does not conflict with checking out the older base.
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0600))

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "msg", SubmitBranchBase(initial))(t.Context(), nil))

		require.Equal(t, initial, gitOutput(t, remote, "rev-parse", "feature/x~1"))
		require.Equal(t, "new.txt", gitOutput(t, remote, "ls-tree", "--name-only", "feature/x"))
	})

	t.Run("message with shell special characters", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0600))
		message := `it's "quoted" $(touch pwned) ` + "`touch pwned`"

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", message)(t.Context(), nil))

		require.Equal(t, message, gitOutput(t, remote, "log", "-1", "--format=%s", "feature/x"))
		require.NoFileExists(t, filepath.Join(dir, "pwned"))
	})

	t.Run("only untracked files count as changes", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("hello"), 0600))

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "msg")(t.Context(), nil))

		require.Equal(t, "untracked.txt", gitOutput(t, remote, "ls-tree", "--name-only", "feature/x"))
	})

	t.Run("modified tracked file is committed", func(t *testing.T) {
		dir, remote := submitbranchfixture(t)
		gitCommitFiles(t, dir, []string{"file.txt"}, "add file")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed"), 0600))

		require.NoError(t, submitbranch(shell.NewLocal(), "feature/x", "edit file")(t.Context(), nil))

		require.Equal(t, "changed", gitOutput(t, remote, "show", "feature/x:file.txt"))
	})

	t.Run("existing branch name fails", func(t *testing.T) {
		dir, _ := submitbranchfixture(t)
		gitOutput(t, dir, "branch", "feature/x")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0600))

		require.Error(t, submitbranch(shell.NewLocal(), "feature/x", "msg")(t.Context(), nil))
	})

	t.Run("blank remote option fails", func(t *testing.T) {
		_, _ = submitbranchfixture(t)

		require.Error(t, submitbranch(shell.NewLocal(), "feature/x", "msg", SubmitBranchRemote(" "))(t.Context(), nil))
	})
}

func TestPushAuth(t *testing.T) {
	t.Run("no token leaves the runtime untouched", func(t *testing.T) {
		cmd := pushauth(shell.Runtime(), " ").New("git push")
		rec := shell.NewRecorder(&cmd)
		require.NoError(t, shell.Run(t.Context(), cmd))
		require.NotContains(t, rec.Result(), "GIT_CONFIG")
	})

	t.Run("token is only in the environment", func(t *testing.T) {
		cmd := pushauth(shell.Runtime(), "sekret").New("git push")
		rec := shell.NewRecorder(&cmd)
		require.NoError(t, shell.Run(t.Context(), cmd))
		require.Contains(t, rec.Result(), "GIT_CONFIG_KEY_0=http.extraheader")
		require.NotContains(t, rec.Result(), "sekret")
	})
}

func TestShquote(t *testing.T) {
	require.Equal(t, "", shquote(""))
	require.Equal(t, `'a b'`, shquote("a b"))
	require.Equal(t, `'it'\''s'`, shquote("it's"))
}
