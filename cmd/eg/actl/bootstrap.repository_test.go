package actl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/egdaemon/eg/cmd/eg/actl"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/require"
)

func TestBootstrapRepository(t *testing.T) {
	t.Run("refuses to overwrite an existing module directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".eg"), 0o700))

		genparser := cmdtestx.Genparser(actl.BootstrapRepository{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg", "vars_git_default_reference": "refs/heads/main"})
		err := cmdtestx.Execute(t, genparser(t), "command", filepath.Join(t.TempDir(), "missing"))
		require.ErrorContains(t, err, "directory already exists")

		var uf interface{ UserFriendly() }
		require.ErrorAs(t, err, &uf)
	})

	t.Run("fails when the repository cannot be cloned", func(t *testing.T) {
		dir := t.TempDir()

		genparser := cmdtestx.Genparser(actl.BootstrapRepository{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg", "vars_git_default_reference": "refs/heads/main"})
		err := cmdtestx.Execute(t, genparser(t), "command", filepath.Join(t.TempDir(), "missing"))
		require.ErrorContains(t, err, "unable to clone repository")
		require.NoDirExists(t, filepath.Join(dir, ".eg", ".git"))
	})

	t.Run("fails when the branch does not exist", func(t *testing.T) {
		src := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "main.go"), []byte("package main\n"), 0o600))
		repo, err := git.PlainInit(src, false)
		require.NoError(t, err)
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add(".")
		require.NoError(t, err)
		_, err = wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "eg", Email: "eg@example.com"}})
		require.NoError(t, err)

		dir := t.TempDir()
		genparser := cmdtestx.Genparser(actl.BootstrapRepository{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg", "vars_git_default_reference": "refs/heads/main"})
		err = cmdtestx.Execute(t, genparser(t), "command", src, "--branch", "refs/heads/does-not-exist")
		require.ErrorContains(t, err, "unable to clone repository")
	})

	t.Run("clones the repository into the module directory without its git history", func(t *testing.T) {
		src := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "main.go"), []byte("package main\n"), 0o600))
		repo, err := git.PlainInit(src, false)
		require.NoError(t, err)
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add(".")
		require.NoError(t, err)
		_, err = wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "eg", Email: "eg@example.com"}})
		require.NoError(t, err)
		head, err := repo.Head()
		require.NoError(t, err)

		// without a git root the workspace is created in the current directory.
		dir := t.TempDir()
		t.Chdir(dir)
		genparser := cmdtestx.Genparser(actl.BootstrapRepository{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg", "vars_git_default_reference": head.Name().String()})
		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", src))

		egdir := filepath.Join(dir, ".eg")
		require.FileExists(t, filepath.Join(egdir, "main.go"))
		require.FileExists(t, filepath.Join(egdir, "go.mod"))
		require.NoDirExists(t, filepath.Join(egdir, ".git"))
	})

	t.Run("registers the module with a workspace at the git root", func(t *testing.T) {
		src := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "main.go"), []byte("package main\n"), 0o600))
		repo, err := git.PlainInit(src, false)
		require.NoError(t, err)
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add(".")
		require.NoError(t, err)
		_, err = wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "eg", Email: "eg@example.com"}})
		require.NoError(t, err)
		head, err := repo.Head()
		require.NoError(t, err)

		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o700))
		genparser := cmdtestx.Genparser(actl.BootstrapRepository{}, kong.Vars{"vars_git_directory": dir, "vars_workload_directory": ".eg", "vars_git_default_reference": head.Name().String()})
		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", src))

		work, err := os.ReadFile(filepath.Join(dir, "go.work"))
		require.NoError(t, err)
		require.Contains(t, string(work), "./.eg")
	})
}
