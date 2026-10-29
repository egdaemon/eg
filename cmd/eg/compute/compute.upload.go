package compute

import (
	"errors"
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
	"github.com/egdaemon/eg/secrets"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/ssh"
)

type upload struct {
	cmdopts.RuntimeResources
	HostedCompute    bool `name:"shared-compute" help:"allow hosted compute" default:"true"`
	uploadoptions    `embed:""`
	SSHKeyPath       string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	Name             string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload"`
	EnvironmentPaths []string `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Dirty            bool     `name:"dirty" help:"include all environment variables"`
	Endpoint         string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	GitClone         string   `name:"git-clone-uri" help:"clone uri"`
	GCPAuto          bool     `name:"gcp-auto" help:"have the runner issue the account's gcp credentials (workload identity) to the workload"`
	Secrets          []string `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
}

// bounds the number of runners we attempt to upload to directly before
// falling back to the cluster.
const directCandidates = 8

func (t upload) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, find libp2px.MDNSLookup) (err error) {
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
		uuid.Must(uuid.NewV7()),
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

	envb := envx.Build().
		Var(eg.EnvComputeArch, t.Arch).
		Var(eg.EnvComputeOS, t.OS).
		FromReader(secrets.NewReader(gctx.Context, t.Secrets...)).
		FromEnviron(envx.Dirty(t.Dirty)...).
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

	if err = tarx.Pack(archiveio, filepath.Join(ws.Root, ws.BuildDir), environio.Name()); err != nil {
		return errorsx.Wrap(err, "unable to pack the kernel archive")
	}

	if err = iox.Rewind(archiveio); err != nil {
		return errorsx.Wrap(err, "unable to rewind kernel archive")
	}

	log.Println("archive", archiveio.Name())
	// if err = tarx.Inspect(archiveio); err != nil {
	// 	log.Println(errorsx.Wrap(err, "unable to inspect archive"))
	// }

	// if err = iox.Rewind(archiveio); err != nil {
	// 	return errorsx.Wrap(err, "unable to rewind kernel archive")
	// }

	ainfo := errorsx.Zero(os.Stat(archiveio.Name()))
	log.Println("archive metadata", ainfo.Name(), bytesx.Unit(ainfo.Size()))

	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)))
	if err != nil {
		return err
	}

	enq := &computeapi.Enqueued{
		Id:          uuid.Must(uuid.NewV7()).String(),
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
		Description: stringsx.FirstNonBlank(t.Name, gitx.Description("Uploaded: %git.commit.author% - %git.ref%", repo, t.GitReference)),
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource)

	return t.submit(gctx.Context, chttp, t.Endpoint, lancache, enq, archiveio, environio)
}
