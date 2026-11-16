package gitx

import (
	"os"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/stretchr/testify/require"
)

func TestUserEnv(t *testing.T) {
	t.Run("configured identity is passed as git config environment", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("GIT_CONFIG_COUNT", "")
		require.NoError(t, os.Unsetenv("GIT_CONFIG_COUNT"))
		repo, err := git.PlainInit(t.TempDir(), false)
		require.NoError(t, err)
		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.User.Name = "Ada"
		cfg.User.Email = "ada@example.com"
		require.NoError(t, repo.SetConfig(cfg))

		require.Equal(t, []string{
			"GIT_CONFIG_COUNT=2",
			"GIT_CONFIG_KEY_0=user.name", "GIT_CONFIG_VALUE_0=Ada",
			"GIT_CONFIG_KEY_1=user.email", "GIT_CONFIG_VALUE_1=ada@example.com",
		}, UserEnv(repo))
	})

	t.Run("no identity yields nothing", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("GIT_CONFIG_COUNT", "")
		require.NoError(t, os.Unsetenv("GIT_CONFIG_COUNT"))
		repo, err := git.PlainInit(t.TempDir(), false)
		require.NoError(t, err)

		require.Empty(t, UserEnv(repo))
	})
}
