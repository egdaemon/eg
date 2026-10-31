package cmdopts_test

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceVCSURI(t *testing.T) {
	t.Run("repository with an origin remote", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)
		_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/egdaemon/eg.git"}})
		require.NoError(t, err)
		t.Chdir(dir)

		require.Equal(t, "git@github.com:egdaemon/eg.git", cmdopts.WorkspaceVCSURI())
	})

	t.Run("repository without a remote is blank and silent", func(t *testing.T) {
		dir := t.TempDir()
		_, err := git.PlainInit(dir, false)
		require.NoError(t, err)
		t.Chdir(dir)

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		require.Equal(t, "", cmdopts.WorkspaceVCSURI())
		require.Empty(t, logged.String())
	})

	t.Run("directory outside of a repository is blank and silent", func(t *testing.T) {
		t.Chdir(t.TempDir())

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		require.Equal(t, "", cmdopts.WorkspaceVCSURI())
		require.Empty(t, logged.String())
	})
}
