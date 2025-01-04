package actl

import (
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/errorsx"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type BootstrapRepository struct {
	Dir      string `name:"directory" help:"root directory to use" default:"${vars_git_directory}"`
	Relative string `name:"relative" help:"relative path from the directory to create the module within" default:"${vars_workload_directory}"`
	URI      string `arg:"" name:"uri" help:"repository uri to clone" default:""`
	Branch   string `name:"branch" help:"name of the branch to clone" default:"${vars_git_default_reference}"`
}

func (t BootstrapRepository) Run(gctx *cmdopts.Global) (err error) {
	egdir := filepath.Join(t.Dir, t.Relative)
	if _, err := os.Stat(egdir); err == nil {
		return errorsx.UserFriendly(errorsx.Errorf("directory already exists, refusing to initialize a new eg module: %s", egdir))
	}

	_, err = git.PlainCloneContext(gctx.Context, filepath.Join(t.Dir, t.Relative), false, &git.CloneOptions{
		URL:               t.URI,
		ReferenceName:     plumbing.ReferenceName(t.Branch),
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Auth:              nil,
		SingleBranch:      true,
	})

	if err != nil {
		return errorsx.Wrap(err, "unable to clone repository")
	}

	if err = os.RemoveAll(filepath.Join(egdir, ".git")); err != nil {
		return errorsx.Wrap(err, "unable to remove upstream git directory")
	}

	if err = compile.InitGolang(gctx.Context, egdir, cmdopts.ModPath()); err != nil {
		return err
	}

	if err = compile.InitGolangTidy(gctx.Context, egdir); err != nil {
		return err
	}

	return nil
}
