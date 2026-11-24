package compute

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/identity"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
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
	"github.com/egdaemon/eg/internal/uuidx"
	"github.com/egdaemon/eg/interp/environ"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/secrets"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
	"golang.org/x/crypto/ssh"
)

type upload struct {
	cmdopts.RuntimeResources
	cmdopts.Publishing
	HostedCompute    bool `name:"shared-compute" help:"allow hosted compute" default:"true"`
	uploadoptions    `embed:""`
	SSHKeyPath       string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	Name             string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload"`
	EnvironmentPaths []string `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Endpoint         string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	GitClone         string   `name:"git-clone-uri" help:"clone uri"`
	GCPAuto          bool     `name:"gcp-auto" help:"have the runner issue the account's gcp credentials (workload identity) to the workload"`
	Secrets          []string `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`

	// the Containerfile the runner builds the workload's container from, shipped in the archive. there are three modes:
	// by default none is shipped and the runner uses its builtin container, --upload-containerfile ships the nearest one
	// from the workload's directory up to the module directory, --containerfile ships the one it names.
	UploadContainerfile bool   `name:"upload-containerfile" help:"ship the nearest Containerfile, from the workload's directory up to the module directory, for the runner to build the workload's container from. by default none is shipped and the runner uses its builtin container" default:"false" hidden:"true"`
	Containerfile       string `name:"containerfile" help:"ship this Containerfile, relative to the repository directory, for the runner to build the workload's container from" hidden:"true"`
}

// containerfile the path of the Containerfile to ship in the archive, empty when none is: the one named by --containerfile,
// the nearest one with --upload-containerfile, and otherwise none so the runner uses its builtin container.
func (t upload) containerfile() (path string, err error) {
	if !t.UploadContainerfile && stringsx.Blank(t.Containerfile) {
		return "", nil
	}

	moduledir := eg.DefaultModuleDirectory(t.Dir)
	path, _, err = runners.LocateContainerfile(t.Dir, moduledir, filepath.Join(moduledir, t.Name), t.Containerfile, t.Name, false)
	return path, err
}

func (t upload) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, find libp2px.MDNSLookup) (err error) {
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

	if ws, err = workspaces.NewLocal(
		gctx.Context,
		uuidx.NewV7(),
		md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())),
		t.Dir,
		t.Name,
	); err != nil {
		return err
	}
	defer os.RemoveAll(ws.Root)

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
		return errorsx.Wrapf(err, "unable to open git repository %s", ws.WorkingDir)
	}

	t.GitClone = stringsx.First(t.GitClone, errorsx.Zero(gitx.QuirkCloneURI(repo, t.GitRemote)))

	// the environment the enclosing workload was provided, everything below overrides it.
	inherited, ierr := environ.Inherited(gctx.Context)
	errorsx.Log(errorsx.Wrap(ierr, "warning: unable to inherit the workload environment, continuing without it"))

	envb := envx.Build().
		FromEnviron(inherited...).
		Var(eg.EnvComputeArch, t.Arch).
		Var(eg.EnvComputeOS, t.OS).
		Var(eg.EnvWorkloadDraft, strconv.FormatBool(t.Draft)).
		Var(eg.EnvWorkloadRelease, strconv.FormatBool(t.Release)).
		FromReader(secrets.NewReader(gctx.Context, t.Secrets...)).
		FromPath(t.EnvironmentPaths...).
		FromEnviron(envx.AutoEnviron(t.Environment...)...).
		FromEnviron(errorsx.Zero(gitx.Env(repo, t.GitRemote, t.GitReference, t.GitClone))...)

	if t.GCPAuto {
		envb = envb.Var(eg.EnvGCPWorkloadIdentityEnabled, "true")
	}

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

	containerfile, err := t.containerfile()
	if err != nil {
		return err
	}

	if err = tarx.Pack(
		gctx.Context,
		archiveio,
		tarx.Paths(filepath.Join(ws.Root, ws.BuildDir), environio.Name()),
		tarx.MaybeFile(containerfile, runners.ArchiveContainerfile),
	); err != nil {
		return errorsx.Wrap(err, "unable to pack the kernel archive")
	}

	debugx.Fn(func() {
		errorsx.Log(errorsx.Wrap(iox.Rewind(archiveio), "unable to rewind kernel archive"))
		errorsx.Log(errorsx.Wrap(tarx.Inspect(gctx.Context, archiveio), "unable to inspect archive"))
	})

	if err = iox.Rewind(archiveio); err != nil {
		return errorsx.Wrap(err, "unable to rewind kernel archive")
	}

	log.Println("archive", archiveio.Name())

	ainfo := errorsx.Zero(os.Stat(archiveio.Name()))
	log.Println("archive metadata", ainfo.Name(), bytesx.Unit(ainfo.Size()))

	aid, err := cmdidentity.AccountID(ident.With(identity.QueryVCSAuto(gctx.AccountID, gctx.ProfileID, errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)))).Resolve(tlsc.DefaultClient(), signer))
	if err != nil {
		return err
	}

	enq := &computeapi.Enqueued{
		Id:          uuidx.NewV7().String(),
		AccountId:   aid,
		Entry:       filepath.Join(ws.Module, filepath.Base(entry.Path)),
		Workload:    t.Name,
		Ttl:         uint64(t.RuntimeResources.TTL.Milliseconds()),
		Cores:       t.RuntimeResources.Cores,
		Memory:      uint64(langx.FirstNonZero(t.RuntimeResources.Memory, 8*bytesx.GiB)),
		Vram:        uint64(t.RuntimeResources.Vram),
		Arch:        t.RuntimeResources.Arch,
		Os:          t.RuntimeResources.OS,
		AllowShared: t.HostedCompute,
		VcsUri:      errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)), // optionally set the vcsuri if we're inside a repository.
		VcsCommit:   errorsx.Zero(gitx.Commitish(ws.WorkingDir, t.GitReference)),
		Labels:      append([]string{}, t.RuntimeResources.Labels...),
		Description: stringsx.FirstNonBlank(t.Name, gitx.Description("Uploaded: {git.commit.author} - {git.ref}", repo, t.GitReference)),
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, gctx.ProfileID, compute.NewAuthzTokenSource)

	return t.submit(gctx.Context, chttp, t.Endpoint, lancache, enq, archiveio, environio)
}
