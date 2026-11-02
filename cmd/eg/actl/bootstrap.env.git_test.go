package actl

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/stretchr/testify/require"
)

func TestBootstrapEnvGitParsing(t *testing.T) {
	t.Run("defaults to the local config of the repository directory", func(t *testing.T) {
		genparser := cmdtestx.Genparser(BootstrapEnvGit{}, kong.Vars{"vars_git_directory": "/expected/repo", "vars_ssh_key_path": "/expected/key", "vars_vcs_uri": ""})

		kctx, err := genparser(t).Parse([]string{"command"})
		require.NoError(t, err)

		var global, directory bool
		for _, flag := range kctx.Selected().Flags {
			switch flag.Name {
			case "global":
				global = flag.Target.Bool()
			case "directory":
				directory = flag.Target.String() == "/expected/repo"
			}
		}
		require.False(t, global)
		require.True(t, directory)
	})

	t.Run("global flag", func(t *testing.T) {
		genparser := cmdtestx.Genparser(BootstrapEnvGit{}, kong.Vars{"vars_git_directory": "/expected/repo", "vars_ssh_key_path": "/expected/key", "vars_vcs_uri": ""})

		kctx, err := genparser(t).Parse([]string{"command", "--global"})
		require.NoError(t, err)

		var global bool
		for _, flag := range kctx.Selected().Flags {
			if flag.Name == "global" {
				global = flag.Target.Bool()
			}
		}
		require.True(t, global)
	})
}

func TestWriteGitConfig(t *testing.T) {
	t.Run("local writes the repository config and leaves the global config alone", func(t *testing.T) {
		global := filepath.Join(t.TempDir(), "gitconfig")
		t.Setenv("GIT_CONFIG_GLOBAL", global)

		dir := t.TempDir()
		require.NoError(t, exec.Command("git", "init", "-q", dir).Run())

		require.NoError(t, writegitconfig(context.Background(), dir, false, "Jane Doe", "jane@example.com"))

		name, err := exec.Command("git", "-C", dir, "config", "--local", "user.name").Output()
		require.NoError(t, err)
		require.Equal(t, "Jane Doe\n", string(name))

		email, err := exec.Command("git", "-C", dir, "config", "--local", "user.email").Output()
		require.NoError(t, err)
		require.Equal(t, "jane@example.com\n", string(email))

		_, err = os.Stat(global)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("global writes the global config and leaves the repository alone", func(t *testing.T) {
		global := filepath.Join(t.TempDir(), "gitconfig")
		t.Setenv("GIT_CONFIG_GLOBAL", global)

		dir := t.TempDir()
		require.NoError(t, exec.Command("git", "init", "-q", dir).Run())

		require.NoError(t, writegitconfig(context.Background(), dir, true, "Jane Doe", "jane@example.com"))

		raw, err := os.ReadFile(global)
		require.NoError(t, err)
		require.Contains(t, string(raw), "name = Jane Doe")
		require.Contains(t, string(raw), "email = jane@example.com")

		require.Error(t, exec.Command("git", "-C", dir, "config", "--local", "user.name").Run())
	})

	t.Run("local outside of a repository points at --global", func(t *testing.T) {
		t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))

		err := writegitconfig(context.Background(), t.TempDir(), false, "Jane Doe", "jane@example.com")
		require.ErrorContains(t, err, "--global")
	})
}

func TestGitUser(t *testing.T) {
	profiles := []*authn.Authn{
		{Account: &authn.Account{Id: "a1"}, Profile: &authn.Profile{Id: "p1", AccountId: "a1", Display: "One", Email: "one@example.com"}},
		{Account: &authn.Account{Id: "a2"}, Profile: &authn.Profile{Id: "p2", AccountId: "a2", Display: "Two", Email: "two@example.com"}},
		{Account: &authn.Account{Id: "a3"}, Profile: &authn.Profile{Id: "p3", AccountId: "a3", Display: "", Email: "three@example.com"}},
		{Account: &authn.Account{Id: "a4"}, Profile: &authn.Profile{Id: "p4", AccountId: "a4", Display: "Four", Email: ""}},
	}

	t.Run("matches the profile id", func(t *testing.T) {
		name, email, err := gituser(&authn.Authn{Account: &authn.Account{Id: "a2"}, Profile: &authn.Profile{Id: "p2", AccountId: "a2"}}, profiles...)
		require.NoError(t, err)
		require.Equal(t, "Two", name)
		require.Equal(t, "two@example.com", email)
	})

	t.Run("falls back to the account when only the account is known", func(t *testing.T) {
		name, email, err := gituser(&authn.Authn{Account: &authn.Account{Id: "a1"}}, profiles...)
		require.NoError(t, err)
		require.Equal(t, "One", name)
		require.Equal(t, "one@example.com", email)
	})

	t.Run("unknown profile fails", func(t *testing.T) {
		_, _, err := gituser(&authn.Authn{Account: &authn.Account{Id: "missing"}, Profile: &authn.Profile{Id: "missing", AccountId: "missing"}}, profiles...)
		require.ErrorContains(t, err, "unable to find")
	})

	t.Run("blank display name fails", func(t *testing.T) {
		_, _, err := gituser(&authn.Authn{Profile: &authn.Profile{Id: "p3", AccountId: "a3"}}, profiles...)
		require.ErrorContains(t, err, "display name")
	})

	t.Run("blank email fails", func(t *testing.T) {
		_, _, err := gituser(&authn.Authn{Profile: &authn.Profile{Id: "p4", AccountId: "a4"}}, profiles...)
		require.ErrorContains(t, err, "email")
	})
}
