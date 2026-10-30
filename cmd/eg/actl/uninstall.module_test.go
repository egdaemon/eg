package actl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/egdaemon/eg/cmd/eg/actl"
	"github.com/stretchr/testify/require"
)

func TestUninstallModule(t *testing.T) {
	t.Run("errors when there is no module", func(t *testing.T) {
		dir := t.TempDir()

		genparser := cmdtestx.Genparser(actl.UninstallModule{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg"})
		err := cmdtestx.Execute(t, genparser(t), "command")
		require.ErrorContains(t, err, "no eg module found")

		var uf interface{ UserFriendly() }
		require.ErrorAs(t, err, &uf)
	})

	t.Run("removes a module containing only generated files", func(t *testing.T) {
		dir := t.TempDir()
		egdir := filepath.Join(dir, ".eg")
		require.NoError(t, os.MkdirAll(egdir, 0o700))
		for _, name := range []string{".gitignore", "go.mod", "go.sum", "main.go"} {
			require.NoError(t, os.WriteFile(filepath.Join(egdir, name), []byte("x"), 0o600))
		}

		genparser := cmdtestx.Genparser(actl.UninstallModule{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg"})
		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command"))
		require.NoDirExists(t, egdir)
	})

	t.Run("refuses to remove a module containing other files without force", func(t *testing.T) {
		dir := t.TempDir()
		egdir := filepath.Join(dir, ".eg")
		require.NoError(t, os.MkdirAll(filepath.Join(egdir, "release"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(egdir, "main.go"), []byte("x"), 0o600))

		genparser := cmdtestx.Genparser(actl.UninstallModule{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg"})
		err := cmdtestx.Execute(t, genparser(t), "command")
		require.ErrorContains(t, err, "use --force")
		require.DirExists(t, egdir)

		var uf interface{ UserFriendly() }
		require.ErrorAs(t, err, &uf)
	})

	t.Run("removes a module containing other files with force", func(t *testing.T) {
		dir := t.TempDir()
		egdir := filepath.Join(dir, ".eg")
		require.NoError(t, os.MkdirAll(filepath.Join(egdir, "release"), 0o700))

		genparser := cmdtestx.Genparser(actl.UninstallModule{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg"})
		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", "--force"))
		require.NoDirExists(t, egdir)
	})
}
