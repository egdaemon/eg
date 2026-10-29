package compute

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

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
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/ssh"
)

type c8scmds struct {
	Upload c8sUpload `cmd:"" help:"upload and run a container file"`
	Local  c8sLocal  `cmd:"" help:"upload and run a container file"`
}

//go:embed .bootstrap.c8s
var embeddedc8supload embed.FS

type c8sUpload struct {
	cmdopts.RuntimeResources
	uploadoptions `embed:""`
	HostedCompute bool     `name:"shared-compute" help:"allow hosted compute" default:"true"`
	Clone         bool     `name:"clone-repo" help:"clone the repository before running the container" default:"false"`
	Containerfile string   `arg:"" help:"path to the container file to run" default:"Containerfile"`
	SSHKeyPath    string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Environment   []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote     string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference  string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	GitClone      string   `name:"git-clone-uri" help:"clone uri"`
	Endpoint      string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
}

func (t c8sUpload) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, find libp2px.MDNSLookup) (err error) {
	const buildir = "build"
	var (
		tmpdir               string
		ws                   workspaces.Context
		signer               ssh.Signer
		repo                 *git.Repository
		archiveio, environio *os.File
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	lancache, stop := t.remember(gctx.Context, find)
	defer stop()

	if repo, err = git.PlainOpen(ws.Root); err != nil {
		return errorsx.Wrap(err, "unable to open git repository")
	}

	if tmpdir, err = os.MkdirTemp("", "eg.c8s.upload.*"); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}

	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	egdir := filepath.Join(tmpdir, eg.DefaultModuleDirectory())
	if err = fsx.MkDirs(0700, egdir, filepath.Join(tmpdir, buildir, "workspace")); err != nil {
		return err
	}

	autoruncontainer := filepath.Join(tmpdir, buildir, "workspace", "Containerfile")

	if err = fsx.CloneTree(gctx.Context, egdir, ".bootstrap.c8s", embeddedc8supload); err != nil {
		return err
	}

	if err = compile.InitGolang(gctx.Context, egdir, cmdopts.ModPath()); err != nil {
		return err
	}

	if err = compile.InitGolangTidy(gctx.Context, egdir); err != nil {
		return err
	}

	if err = iox.Copy(t.Containerfile, autoruncontainer); err != nil {
		return err
	}

	if ws, err = workspaces.NewBuiltin(
		gctx.Context,
		uuid.Must(uuid.NewV7()),
		md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())),
		tmpdir,
		""); err != nil {
		return err
	}

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(tmpdir), ws)).Run(gctx.Context)
	if err != nil {
		return err
	}

	if err = compile.EnsureRequiredPackages(gctx.Context, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return errorsx.Wrap(err, "unable to transpile")
	}

	entry, found := slicesx.Find(func(c transpile.Compiled) bool {
		return !c.Generated
	}, modules...)

	if !found {
		return errors.New("unable to locate entry point")
	}

	if environio, err = os.Create(filepath.Join(tmpdir, eg.EnvironFile)); err != nil {
		return errorsx.Wrap(err, "unable to open the kernel archive")
	}
	defer environio.Close()

	envb := envx.Build().
		FromEnviron(t.Environment...).
		FromEnviron(errorsx.Zero(gitx.Env(repo, t.GitRemote, t.GitReference, t.GitClone))...).
		Var(eg.EnvComputeContainerImpure, strconv.FormatBool(t.Clone))

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to write environment variables buffer")
	}

	if err = iox.Rewind(environio); err != nil {
		return errorsx.Wrap(err, "unable to rewind environment variables buffer")
	}

	debugx.Println(envx.PrintEnv(errorsx.Zero(envb.Environ())...))

	if archiveio, err = os.CreateTemp(tmpdir, "kernel.*.tar.gz"); err != nil {
		return errorsx.Wrap(err, "unable to open the kernel archive")
	}
	defer archiveio.Close()

	if err = tarx.Pack(archiveio, filepath.Join(ws.Root, ws.BuildDir), environio.Name(), filepath.Join(tmpdir, buildir)); err != nil {
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
	log.Println("archive metadata", ainfo.Name(), ainfo.Size())

	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)))
	if err != nil {
		return err
	}

	enq := &computeapi.Enqueued{
		Id:          uuid.Must(uuid.NewV7()).String(),
		AccountId:   aid,
		Description: fmt.Sprintf("container workload - %s", t.Containerfile),
		AllowShared: t.HostedCompute,
		Entry:       filepath.Base(entry.Path),
		Ttl:         uint64(t.RuntimeResources.TTL.Milliseconds()),
		Cores:       t.RuntimeResources.Cores,
		Memory:      uint64(langx.FirstNonZero(t.RuntimeResources.Memory, 8*bytesx.GiB)),
		Arch:        t.RuntimeResources.Arch,
		Os:          t.RuntimeResources.OS,
		VcsUri:      errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)), // optionally set the vcsuri if we're inside a repository.
		VcsCommit:   errorsx.Zero(gitx.Commitish(".", t.GitReference)),  // the server rejects a vcsuri without a commit.
		Labels:      append([]string{}, t.RuntimeResources.Labels...),
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource)

	return t.submit(gctx.Context, chttp, t.Endpoint, lancache, enq, archiveio, environio)
}
