package actl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/egdaemon/eg/cmd/eg/actl"
	"github.com/go-git/go-git/v6"
	"github.com/stretchr/testify/require"
)

func TestBootstrapModule(t *testing.T) {
	t.Run("refuses to overwrite an existing module directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".eg"), 0o700))

		genparser := cmdtestx.Genparser(actl.BootstrapModule{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg"})
		err := cmdtestx.Execute(t, genparser(t), "command")
		require.ErrorContains(t, err, "directory already exists")

		var uf interface{ UserFriendly() }
		require.ErrorAs(t, err, &uf)
	})

	t.Run("creates the module within a repository that has no remote", func(t *testing.T) {
		dir := t.TempDir()
		_, err := git.PlainInit(dir, false)
		require.NoError(t, err)
		t.Chdir(dir)

		genparser := cmdtestx.Genparser(actl.BootstrapModule{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg"})
		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command"))
		require.FileExists(t, filepath.Join(dir, ".eg", "go.mod"))
	})
}
