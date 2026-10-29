package compute

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/internal/unsafepretty"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/ssh"
)

type builtinUpload struct {
	cmdopts.RuntimeResources
	uploadoptions `embed:""`
	HostedCompute bool     `name:"shared-compute" help:"allow hosted compute" default:"true"`
	SSHKeyPath    string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Dir           string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	Name          string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload.builtin"`
	Environment   []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Dirty         bool     `name:"dirty" help:"include all environment variables"`
	Endpoint      string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
	GitRemote     string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference  string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	GitClone      string   `name:"git-clone-uri" help:"clone uri"`
}

func (t builtinUpload) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, find libp2px.MDNSLookup) (err error) {
	var (
		signer               ssh.Signer
		ws                   workspaces.Context
		repo                 *git.Repository
		tmpdir               string
		archiveio, environio *os.File
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	lancache, stop := t.remember(gctx.Context, find)
	defer stop()

	if tmpdir, err = os.MkdirTemp("", "eg.builtin.*"); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}
	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if ws, err = workspaces.NewBuiltin(
		gctx.Context,
		uuid.Must(uuid.NewV7()),
		md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())),
		tmpdir,
		t.Name,
		workspaces.OptionSymlinkCache(filepath.Join(t.Dir, eg.CacheDirectory)),
		workspaces.OptionSymlinkWorking(t.Dir),
	); err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Join(ws.Root, ws.RuntimeDir))

	if err = fsx.CloneTree(gctx.Context, eg.DefaultModuleDirectory(ws.Root), ".builtin", embeddedbuiltin); err != nil {
		return errorsx.Wrap(err, "unable to clone tree")
	}

	if err = compile.InitGolang(gctx.Context, eg.DefaultModuleDirectory(ws.Root), cmdopts.ModPath()); err != nil {
		return errorsx.Wrap(err, "unable to init golang module")
	}

	if err = compile.InitGolangTidy(gctx.Context, eg.DefaultModuleDirectory(ws.Root)); err != nil {
		return errorsx.Wrap(err, "unable to tidy up dependency")
	}

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(ws.Root), ws)).Run(gctx.Context)
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

	if repo, err = git.PlainOpen("."); err != nil {
		return errorsx.Wrapf(err, "unable to open git repository %s", ws.WorkingDir)
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

	ainfo := errorsx.Zero(os.Stat(archiveio.Name()))
	log.Println("archive metadata", ainfo.Name(), bytesx.Unit(ainfo.Size()))

	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)))
	if err != nil {
		return err
	}

	enq := &computeapi.Enqueued{
		Id:          uuid.Must(uuid.NewV7()).String(),
		AccountId:   aid,
		Description: fmt.Sprintf("builtin workload - %s", t.Name),
		Entry:       filepath.Join(ws.Module, filepath.Base(entry.Path)),
		Workload:    t.Name,
		Ttl:         uint64(t.RuntimeResources.TTL.Milliseconds()),
		Cores:       t.RuntimeResources.Cores,
		Memory:      uint64(langx.FirstNonZero(t.RuntimeResources.Memory, 8*bytesx.GiB)),
		Arch:        t.RuntimeResources.Arch,
		Os:          t.RuntimeResources.OS,
		AllowShared: t.HostedCompute,
		VcsUri:      errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)), // optionally set the vcsuri if we're inside a repository.
		VcsCommit:   errorsx.Zero(gitx.Commitish(".", t.GitReference)),  // the server rejects a vcsuri without a commit.
		Labels:      append([]string{}, t.RuntimeResources.Labels...),
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource)

	return t.submit(gctx.Context, chttp, t.Endpoint, lancache, enq, archiveio, environio)
}
