package eggit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/runtime/wasi/eg"
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

func changefixture(t *testing.T) (dir, remote string) {
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
	t.Setenv(_eg.EnvComputeWorkspaceDirectory, dir)
	t.Chdir(dir)
	return dir, remote
}

// a change in a worktree outside of the repository using the local shell.
func testchange(t *testing.T) change {
	t.Helper()
	c := Change(filepath.Join(t.TempDir(), "wt"))
	c.runtime = shell.NewLocal()
	return c
}

func noop(Changer) eg.OpFn {
	return func(context.Context, eg.Op) error { return nil }
}

func TestChange(t *testing.T) {
	t.Run("relative paths are within the workspace worktrees directory and absolute paths are used as is", func(t *testing.T) {
		dir, _ := changefixture(t)
		require.Equal(t, filepath.Join(dir, ".eg", ".worktrees", md5x.String("wt")), Change("wt").dir)
		require.Equal(t, "/abs/wt", Change("/abs/wt").dir)
	})

	t.Run("op provides a worktree and reuses it", func(t *testing.T) {
		dir, _ := changefixture(t)
		c := testchange(t)

		var got Changer
		for range 2 {
			require.NoError(t, c.Op(func(wt Changer) eg.OpFn { got = wt; return noop(wt) })(t.Context(), nil))
		}

		require.Equal(t, c.dir, got.Path())
		require.Equal(t, c.dir, gitOutput(t, got.Path(), "rev-parse", "--show-toplevel"))
		require.Contains(t, gitOutput(t, dir, "worktree", "list"), c.dir)
	})

	t.Run("recreates a worktree whose directory was removed", func(t *testing.T) {
		changefixture(t)
		c := testchange(t)
		require.NoError(t, c.ensure(t.Context()))
		require.NoError(t, os.RemoveAll(c.dir))

		require.NoError(t, c.ensure(t.Context()))
		require.DirExists(t, c.dir)
	})

	t.Run("concurrent changes are isolated", func(t *testing.T) {
		dir, remote := changefixture(t)
		a, b := testchange(t), testchange(t)
		require.NoError(t, a.ensure(t.Context()))
		require.NoError(t, b.ensure(t.Context()))
		require.NoError(t, os.WriteFile(filepath.Join(a.dir, "a.txt"), []byte("a"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(b.dir, "b.txt"), []byte("b"), 0600))

		require.NoError(t, Commit(a, NewCommit("a").Branch("feature/a"))(t.Context(), nil))
		require.NoError(t, Commit(b, NewCommit("b").Branch("feature/b"))(t.Context(), nil))
		require.NoError(t, Push(a)(t.Context(), nil))
		require.NoError(t, Push(b)(t.Context(), nil))

		require.Equal(t, "a.txt", gitOutput(t, remote, "ls-tree", "--name-only", "feature/a"))
		require.Equal(t, "b.txt", gitOutput(t, remote, "ls-tree", "--name-only", "feature/b"))
		require.NoFileExists(t, filepath.Join(dir, "a.txt"))
	})

	t.Run("base option creates the worktree from the ref", func(t *testing.T) {
		dir, remote := changefixture(t)
		initial := gitRevParse(t, dir, "HEAD")
		gitCommitFiles(t, dir, []string{"later.txt"}, "later commit")
		c := testchange(t).Base(initial)
		require.NoError(t, c.ensure(t.Context()))
		require.NoError(t, os.WriteFile(filepath.Join(c.dir, "new.txt"), []byte("hello"), 0600))

		require.NoError(t, Commit(c, NewCommit("msg").Branch("feature/x"))(t.Context(), nil))
		require.NoError(t, Push(c)(t.Context(), nil))

		require.Equal(t, initial, gitOutput(t, remote, "rev-parse", "feature/x~1"))
	})
}

func TestCommit(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		for name, tc := range map[string]struct{ branch, message string }{
			"empty branch":   {"", "msg"},
			"blank branch":   {"  ", "msg"},
			"empty message":  {"feature", ""},
			"invalid branch": {"bad..name", "msg"},
		} {
			t.Run(name, func(t *testing.T) {
				changefixture(t)
				require.Error(t, Commit(testchange(t), NewCommit(tc.message).Branch(tc.branch))(t.Context(), nil))
			})
		}
	})

	t.Run("commits worktree changes and leaves the checkout untouched", func(t *testing.T) {
		dir, _ := changefixture(t)
		c := testchange(t)
		require.NoError(t, c.ensure(t.Context()))
		require.NoError(t, os.WriteFile(filepath.Join(c.dir, "new.txt"), []byte("hello"), 0600))
		original := gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD")

		require.NoError(t, Commit(c, NewCommit("add new file\n\nbody").Branch("feature/x"))(t.Context(), nil))

		require.Equal(t, "add new file", gitOutput(t, c.dir, "log", "-1", "--format=%s"))
		require.Equal(t, "feature/x", gitOutput(t, c.dir, "rev-parse", "--abbrev-ref", "HEAD"))
		require.Equal(t, original, gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	})

	t.Run("add copies workspace paths into the worktree", func(t *testing.T) {
		dir, _ := changefixture(t)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "in.txt"), []byte("in"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "out.txt"), []byte("out"), 0600))
		c := testchange(t)

		require.NoError(t, Commit(c, NewCommit("msg").Branch("feature/x").Add("sub/in.txt"))(t.Context(), nil))

		require.Equal(t, "sub/in.txt", gitOutput(t, c.dir, "ls-tree", "-r", "--name-only", "HEAD", "sub"))
		require.NoFileExists(t, filepath.Join(c.dir, "out.txt"))
	})

	t.Run("clean worktree is a noop", func(t *testing.T) {
		_, remote := changefixture(t)
		c := testchange(t)

		require.NoError(t, Commit(c, NewCommit("msg").Branch("feature/x"))(t.Context(), nil))
		require.NoError(t, Push(c)(t.Context(), nil))

		require.Empty(t, gitOutput(t, remote, "branch", "--list"))
		require.Equal(t, "HEAD", gitOutput(t, c.dir, "rev-parse", "--abbrev-ref", "HEAD"))
	})

	t.Run("author", func(t *testing.T) {
		for name, tc := range map[string]struct{ author, email, expected string }{
			"head commit author": {"Jane Doe", "jane@example.com", "Jane Doe <jane@example.com>"},
			"egdaemon fallback":  {"", "", "egdaemon <noreply@egdaemon.com>"},
			"per field":          {"Jane Doe", "", "Jane Doe <noreply@egdaemon.com>"},
		} {
			t.Run(name, func(t *testing.T) {
				changefixture(t)
				t.Setenv(_eg.EnvGitHeadCommitAuthor, tc.author)
				t.Setenv(_eg.EnvGitHeadCommitEmail, tc.email)
				c := testchange(t)
				require.NoError(t, c.ensure(t.Context()))
				require.NoError(t, os.WriteFile(filepath.Join(c.dir, "new.txt"), []byte("hello"), 0600))

				require.NoError(t, Commit(c, NewCommit("msg").Branch("feature/x"))(t.Context(), nil))

				require.Equal(t, tc.expected, gitOutput(t, c.dir, "log", "-1", "--format=%an <%ae>"))
				require.Equal(t, tc.expected, gitOutput(t, c.dir, "log", "-1", "--format=%cn <%ce>"))
			})
		}
	})

	t.Run("message with shell special characters", func(t *testing.T) {
		dir, _ := changefixture(t)
		c := testchange(t)
		require.NoError(t, c.ensure(t.Context()))
		require.NoError(t, os.WriteFile(filepath.Join(c.dir, "new.txt"), []byte("hello"), 0600))
		message := `it's "quoted" $(touch pwned) ` + "`touch pwned`"

		require.NoError(t, Commit(c, NewCommit(message).Branch("feature/x"))(t.Context(), nil))

		require.Equal(t, message, gitOutput(t, c.dir, "log", "-1", "--format=%s"))
		require.NoFileExists(t, filepath.Join(dir, "pwned"))
		require.NoFileExists(t, filepath.Join(c.dir, "pwned"))
	})
}

func TestPush(t *testing.T) {
	t.Run("pushes the committed branch without leaking the token", func(t *testing.T) {
		dir, remote := changefixture(t)
		c := testchange(t)
		require.NoError(t, c.ensure(t.Context()))
		require.NoError(t, os.WriteFile(filepath.Join(c.dir, "new.txt"), []byte("hello"), 0600))
		require.NoError(t, Commit(c, NewCommit("add new file").Branch("feature/x"))(t.Context(), nil))

		require.NoError(t, Push(c)(t.Context(), nil))

		require.Equal(t, "add new file", gitOutput(t, remote, "log", "-1", "--format=%s", "feature/x"))
		require.NotContains(t, gitOutput(t, dir, "config", "--local", "--list"), "sekret")
	})

	t.Run("remote option", func(t *testing.T) {
		dir, _ := changefixture(t)
		other := t.TempDir()
		cmd := exec.Command("git", "init", "--bare")
		cmd.Dir = other
		require.NoError(t, cmd.Run())
		gitOutput(t, dir, "remote", "add", "other", other)
		c := testchange(t)
		require.NoError(t, c.ensure(t.Context()))
		require.NoError(t, os.WriteFile(filepath.Join(c.dir, "new.txt"), []byte("hello"), 0600))
		require.NoError(t, Commit(c, NewCommit("msg").Branch("feature/x"))(t.Context(), nil))

		require.NoError(t, Push(c, PushRemote("other"))(t.Context(), nil))

		require.Equal(t, "feature/x", gitOutput(t, other, "branch", "--list", "--format=%(refname:short)"))
	})

	t.Run("blank remote option fails", func(t *testing.T) {
		changefixture(t)
		require.Error(t, Push(testchange(t), PushRemote(" "))(t.Context(), nil))
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

	t.Run("writes a patch carrying the commit instead of pushing when cloning is disabled", func(t *testing.T) {
		dir, remote := changefixture(t)
		workspace := t.TempDir()
		t.Setenv(_eg.EnvUnsafeGitCloneEnabled, "false")
		t.Setenv(_eg.EnvComputeWorkspaceDirectory, workspace)
		t.Setenv(_eg.EnvGitHeadCommitAuthor, "Jane Doe")
		t.Setenv(_eg.EnvGitHeadCommitEmail, "jane@example.com")
		c := testchange(t)
		require.NoError(t, c.ensure(t.Context()))
		require.NoError(t, os.WriteFile(filepath.Join(c.dir, "one.txt"), []byte("one\n"), 0600))
		require.NoError(t, Commit(c, NewCommit("first").Branch("feature/x"))(t.Context(), nil))
		require.NoError(t, os.WriteFile(filepath.Join(c.dir, "two.txt"), []byte("two\n"), 0600))
		require.NoError(t, Commit(c, NewCommit("second\n\nbody").Branch("feature/x"))(t.Context(), nil))

		require.NoError(t, Push(c)(t.Context(), nil))

		patch := filepath.Join(workspace, ".eg", ".patches", md5x.String("feature/x")+".patch")
		contents, err := os.ReadFile(patch)
		require.NoError(t, err)
		require.Contains(t, string(contents), "Subject: [PATCH 1/2] first")
		require.Contains(t, string(contents), "Subject: [PATCH 2/2] second")
		require.Empty(t, gitOutput(t, remote, "branch", "--list"))

		// the whole commits, message and author included, apply to a pristine clone.
		// am needs a committer, which the environment (e.g. a workload) may not configure.
		t.Setenv("GIT_COMMITTER_NAME", "committer")
		t.Setenv("GIT_COMMITTER_EMAIL", "committer@example.com")
		clone := filepath.Join(t.TempDir(), "clone")
		gitOutput(t, dir, "clone", dir, clone)
		gitOutput(t, clone, "am", patch)
		require.Equal(t, "second", gitOutput(t, clone, "log", "-1", "--format=%s"))
		require.Equal(t, "Jane Doe <jane@example.com>", gitOutput(t, clone, "log", "-1", "--format=%an <%ae>"))
	})

	t.Run("writes no patch without changes when cloning is disabled", func(t *testing.T) {
		changefixture(t)
		workspace := t.TempDir()
		t.Setenv(_eg.EnvUnsafeGitCloneEnabled, "false")
		t.Setenv(_eg.EnvComputeWorkspaceDirectory, workspace)
		c := testchange(t)

		require.NoError(t, Commit(c, NewCommit("msg").Branch("feature/x"))(t.Context(), nil))
		require.NoError(t, Push(c)(t.Context(), nil))

		require.NoDirExists(t, filepath.Join(workspace, ".eg", ".patches"))
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
