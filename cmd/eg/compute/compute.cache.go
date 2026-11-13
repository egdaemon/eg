package compute

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/identity"
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

const (
	// name of the payload archive within the kernel archive, must match .builtin/cache/insert.
	cachePayloadArchive = "cache.payload.tar.gz"
	// environment variable listing the paths to remove, must match .builtin/cache/delete.
	cacheEnvDeletePaths = "EG_CACHE_DELETE_PATHS"
)

type cmdCache struct {
	Insert cacheInsert `cmd:"" help:"upload files and directories into the runner's workload cache, directories are inserted by their contents"`
	Delete cacheDelete `cmd:"" help:"remove paths, relative to the cache root, from the runner's workload cache"`
}

// cacheoptions are the flags shared by the cache commands, the same as builtin upload minus the module.
type cacheoptions struct {
	cmdopts.RuntimeResources
	uploadoptions `embed:""`
	HostedCompute bool     `name:"shared-compute" help:"allow hosted compute" default:"true"`
	SSHKeyPath    string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Dir           string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	Environment   []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Endpoint      string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
	GitRemote     string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference  string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	GitClone      string   `name:"git-clone-uri" help:"clone uri"`
}

// run compiles the embedded builtin module and enqueues it, mirroring builtin upload. extra files are packed
// into the root of the kernel archive and environ is added to the workload's environment.
func (t cacheoptions) run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, find libp2px.MDNSLookup, module string, description string, environ []string, extra ...string) (err error) {
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
		module,
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

	if err = compile.EnsureRequiredPackages(gctx.Context, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return err
	}

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
		return errorsx.Wrap(err, "unable to open the environment file")
	}
	defer environio.Close()

	if repo, err = git.PlainOpen("."); err != nil {
		return errorsx.Wrapf(err, "unable to open git repository %s", ws.WorkingDir)
	}

	t.GitClone = stringsx.First(t.GitClone, errorsx.Zero(gitx.QuirkCloneURI(repo, t.GitRemote)))

	envb := envx.Build().
		FromEnviron(t.Environment...).
		FromEnviron(errorsx.Zero(gitx.Env(repo, t.GitRemote, t.GitReference, t.GitClone))...).
		FromEnviron(environ...)

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

	if err = tarx.Pack(archiveio, append([]string{filepath.Join(ws.Root, ws.BuildDir), environio.Name()}, extra...)...); err != nil {
		return errorsx.Wrap(err, "unable to pack the kernel archive")
	}

	if err = iox.Rewind(archiveio); err != nil {
		return errorsx.Wrap(err, "unable to rewind kernel archive")
	}

	ainfo := errorsx.Zero(os.Stat(archiveio.Name()))
	log.Println("archive metadata", ainfo.Name(), bytesx.Unit(ainfo.Size()))

	aid, err := cmdidentity.AccountID(ident.With(identity.QueryVCSAuto(gctx.AccountID, gctx.ProfileID, errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)))).Resolve(tlsc.DefaultClient(), signer))
	if err != nil {
		return err
	}

	enq := &computeapi.Enqueued{
		Id:          uuid.Must(uuid.NewV7()).String(),
		AccountId:   aid,
		Description: description,
		Entry:       filepath.Join(ws.Module, filepath.Base(entry.Path)),
		Workload:    module,
		Ttl:         uint64(t.RuntimeResources.TTL.Milliseconds()),
		Cores:       t.RuntimeResources.Cores,
		Memory:      uint64(langx.FirstNonZero(t.RuntimeResources.Memory, 8*bytesx.GiB)),
		Arch:        t.RuntimeResources.Arch,
		Os:          t.RuntimeResources.OS,
		AllowShared: t.HostedCompute,
		VcsUri:      errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)),
		VcsCommit:   errorsx.Zero(gitx.Commitish(".", t.GitReference)),
		Labels:      append([]string{}, t.RuntimeResources.Labels...),
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, gctx.ProfileID, compute.NewAuthzTokenSource)

	return t.submit(gctx.Context, chttp, t.Endpoint, lancache, enq, archiveio, environio)
}
