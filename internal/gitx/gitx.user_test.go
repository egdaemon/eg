package gitx

import (
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v6"
	"github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
	t.Run("reads the global config outside of a repository", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = global\n\temail = global@example.com\n"), 0600))

		name, email := User(t.TempDir())
		require.Equal(t, "global", name)
		require.Equal(t, "global@example.com", email)
	})

	t.Run("the repository config overrides the global config", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = global\n\temail = global@example.com\n"), 0600))

		dir := t.TempDir()
		repo, err := gogit.PlainInit(dir, false)
		require.NoError(t, err)
		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.User.Name = "local"
		require.NoError(t, repo.SetConfig(cfg))

		name, email := User(dir)
		require.Equal(t, "local", name)
		require.Equal(t, "global@example.com", email)
	})

	t.Run("blank when nothing is configured", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

		name, email := User(t.TempDir())
		require.Empty(t, name)
		require.Empty(t, email)
	})
}
