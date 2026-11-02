package gitx

import (
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v6"
	"github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
	t.Run("the environment overrides the repository and global config", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		t.Setenv("GIT_AUTHOR_NAME", "author")
		t.Setenv("GIT_AUTHOR_EMAIL", "author@example.com")
		t.Setenv("GIT_COMMITTER_NAME", "")
		t.Setenv("GIT_COMMITTER_EMAIL", "")
		require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = global\n\temail = global@example.com\n"), 0600))

		dir := t.TempDir()
		repo, err := gogit.PlainInit(dir, false)
		require.NoError(t, err)
		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.User.Name = "local"
		cfg.User.Email = "local@example.com"
		require.NoError(t, repo.SetConfig(cfg))

		name, email := User(dir)
		require.Equal(t, "author", name)
		require.Equal(t, "author@example.com", email)
	})

	t.Run("the committer environment is used when the author is unset", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		t.Setenv("GIT_AUTHOR_NAME", "")
		t.Setenv("GIT_AUTHOR_EMAIL", "")
		t.Setenv("GIT_COMMITTER_NAME", "committer")
		t.Setenv("GIT_COMMITTER_EMAIL", "committer@example.com")

		dir := t.TempDir()
		_, err := gogit.PlainInit(dir, false)
		require.NoError(t, err)

		name, email := User(dir)
		require.Equal(t, "committer", name)
		require.Equal(t, "committer@example.com", email)
	})

	t.Run("the author environment beats the committer environment", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		t.Setenv("GIT_AUTHOR_NAME", "author")
		t.Setenv("GIT_AUTHOR_EMAIL", "author@example.com")
		t.Setenv("GIT_COMMITTER_NAME", "committer")
		t.Setenv("GIT_COMMITTER_EMAIL", "committer@example.com")

		dir := t.TempDir()
		_, err := gogit.PlainInit(dir, false)
		require.NoError(t, err)

		name, email := User(dir)
		require.Equal(t, "author", name)
		require.Equal(t, "author@example.com", email)
	})

	t.Run("the name and email fall back to config independently", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		t.Setenv("GIT_AUTHOR_NAME", "author")
		t.Setenv("GIT_AUTHOR_EMAIL", "")
		t.Setenv("GIT_COMMITTER_NAME", "")
		t.Setenv("GIT_COMMITTER_EMAIL", "")
		require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = global\n\temail = global@example.com\n"), 0600))

		dir := t.TempDir()
		_, err := gogit.PlainInit(dir, false)
		require.NoError(t, err)

		name, email := User(dir)
		require.Equal(t, "author", name)
		require.Equal(t, "global@example.com", email)
	})

	t.Run("reads the global config outside of a repository", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("GIT_AUTHOR_NAME", "")
		t.Setenv("GIT_AUTHOR_EMAIL", "")
		t.Setenv("GIT_COMMITTER_NAME", "")
		t.Setenv("GIT_COMMITTER_EMAIL", "")
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = global\n\temail = global@example.com\n"), 0600))

		name, email := User(t.TempDir())
		require.Equal(t, "global", name)
		require.Equal(t, "global@example.com", email)
	})

	t.Run("the repository config overrides the global config", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("GIT_AUTHOR_NAME", "")
		t.Setenv("GIT_AUTHOR_EMAIL", "")
		t.Setenv("GIT_COMMITTER_NAME", "")
		t.Setenv("GIT_COMMITTER_EMAIL", "")
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
		t.Setenv("GIT_AUTHOR_NAME", "")
		t.Setenv("GIT_AUTHOR_EMAIL", "")
		t.Setenv("GIT_COMMITTER_NAME", "")
		t.Setenv("GIT_COMMITTER_EMAIL", "")
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

		name, email := User(t.TempDir())
		require.Empty(t, name)
		require.Empty(t, email)
	})
}
