package compute

import (
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/internal/unsafepretty"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
)

type archive struct {
	Dir          string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	Environment  []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Dirty        bool     `name:"dirty" help:"include all host environment variables"`
	GitRemote    string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	GitClone     string   `name:"git-clone-uri" help:"clone uri"`
	Output       string   `name:"output" short:"o" help:"file to store archive to. default is to write to stdout" default:"-"`
	Name         string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload"`
}

func (t archive) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		ws                   workspaces.Context
		repo                 *git.Repository
		tmpdir               string
		archiveio, environio *os.File
	)

	if ws, err = workspaces.NewLocal(
		gctx.Context, md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())), t.Dir, t.Name,
		workspaces.OptionSymlinkCache(filepath.Join(t.Dir, eg.CacheDirectory)),
		workspaces.OptionSymlinkWorking(t.Dir),
	); err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Join(ws.Root, ws.RuntimeDir))

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(t.Dir), ws)).Run(gctx.Context)
	if err != nil {
		return err
	}

	log.Println("cacheid", ws.CachedID)

	if err = compile.EnsureRequiredPackages(gctx.Context, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return err
	}
	log.Println("modules", modules)

	entry, found := slicesx.Find(func(c transpile.Compiled) bool {
		return !c.Generated
	}, modules...)

	if !found {
		return errors.New("unable to locate entry point")
	}

	if tmpdir, err = os.MkdirTemp("", "eg.upload.*"); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}

	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if environio, err = os.Create(filepath.Join(tmpdir, eg.EnvironFile)); err != nil {
		return errorsx.Wrap(err, "unable to open the kernel archive")
	}
	defer environio.Close()

	if repo, err = git.PlainOpen(ws.WorkingDir); err != nil {
		return errorsx.Wrapf(err, "unable to open git repository: %s", ws.WorkingDir)
	}

	t.GitClone = stringsx.First(t.GitClone, errorsx.Zero(gitx.QuirkCloneURI(repo, t.GitRemote)))

	envb := envx.Build().
		FromEnviron(envx.Dirty(t.Dirty)...).
		FromEnviron(t.Environment...).
		FromEnviron(errorsx.Zero(gitx.Env(repo, t.GitRemote, t.GitReference, t.GitClone))...)

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to write environment variables buffer")
	}

	if err = iox.Rewind(environio); err != nil {
		return errorsx.Wrap(err, "unable to rewind environment variables buffer")
	}

	debugx.Printf("environment\n%s\n", unsafepretty.Print(iox.String(environio), unsafepretty.OptionDisplaySpaces()))

	if archiveio, err = os.CreateTemp(tmpdir, "kernel.*.tar.gz"); err != nil {
		return errorsx.Wrap(err, "unable to open the kernel archive")
	}
	defer archiveio.Close()

	if err = tarx.Pack(archiveio, filepath.Join(ws.Root, ws.BuildDir), environio.Name()); err != nil {
		return errorsx.Wrap(err, "unable to pack the kernel archive")
	}

	if err = iox.Rewind(archiveio); err != nil {
		return errorsx.Wrap(err, "unable to rewind kernel archive")
	}

	log.Println("archive", archiveio.Name())
	if err = tarx.Inspect(archiveio); err != nil {
		log.Println(errorsx.Wrap(err, "unable to inspect archive"))
	}

	if err = iox.Rewind(archiveio); err != nil {
		return errorsx.Wrap(err, "unable to rewind kernel archive")
	}

	ainfo := errorsx.Zero(os.Stat(archiveio.Name()))
	log.Println("archive metadata", ainfo.Name(), bytesx.Unit(ainfo.Size()))

	var dst io.WriteCloser = os.Stdout
	if stringsx.Present(t.Output) && t.Output != "-" {
		if dst, err = os.Create(t.Output); err != nil {
			return errorsx.Wrap(err, "unable to create archive")
		}
	}

	if _, err = io.Copy(dst, archiveio); err != nil {
		return errorsx.Wrap(err, "unable to create archive")
	}

	log.Println("entry point", entry.Path)
	return nil
}
