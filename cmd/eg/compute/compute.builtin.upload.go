package compute

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
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
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/internal/unsafepretty"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type builtinUpload struct {
	cmdopts.RuntimeResources
	HostedCompute bool     `name:"shared-compute" help:"allow hosted compute" default:"true"`
	SSHKeyPath    string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Dir           string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	Name          string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload"`
	Environment   []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Dirty         bool     `name:"dirty" help:"include all environment variables"`
	Endpoint      string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
	GitRemote     string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference  string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	GitClone      string   `name:"git-clone-uri" help:"clone uri"`
}

func (t builtinUpload) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		signer               ssh.Signer
		ws                   workspaces.Context
		repo                 *git.Repository
		tmpdir               string
		archiveio, environio *os.File
		e                    runners.EnqueuedCreateResponse
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	if tmpdir, err = os.MkdirTemp("", "eg.builtin.*"); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}
	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if ws, err = workspaces.NewLocal(
		gctx.Context, md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())), tmpdir, t.Name,
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
		return err
	}

	if err = compile.InitGolangTidy(gctx.Context, eg.DefaultModuleDirectory(ws.Root)); err != nil {
		return err
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

	if repo, err = git.PlainOpen(ws.WorkingDir); err != nil {
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

	// TODO: determine the destination based on the requirements
	// i.e. cores, memory, labels, disk, videomem, etc.
	// not sure if the client should do this or the node we upload to.
	// if its the node we upload to it'll cost more due to having to
	// push the archive to another node that matches the requirements.
	// in theory we could use redirects to handle that but it'd still take a performance hit.
	mimetype, buf, err := runners.NewEnqueueUpload(&runners.Enqueued{
		Entry:       filepath.Join(ws.Module, filepath.Base(entry.Path)),
		Ttl:         uint64(t.RuntimeResources.TTL.Milliseconds()),
		Cores:       t.RuntimeResources.Cores,
		Memory:      uint64(t.RuntimeResources.Memory),
		Arch:        t.RuntimeResources.Arch,
		Os:          t.RuntimeResources.OS,
		AllowShared: t.HostedCompute,
		VcsUri:      errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)), // optionally set the vcsuri if we're inside a repository.
		Labels:      append([]string{}, t.RuntimeResources.Labels...),
	}, archiveio)
	if err != nil {
		return errorsx.Wrap(err, "unable to generate multipart upload")
	}
	defer buf.Close()

	c := tlsc.DefaultClient()
	tokensrc := compute.NewAuthzTokenSource(tlsc.DefaultClient(), signer, authn.EndpointCompute())
	chttp := oauth2.NewClient(
		context.WithValue(gctx.Context, oauth2.HTTPClient, c),
		tokensrc,
	)

	ctx, done := context.WithTimeout(gctx.Context, 10*time.Second)
	defer done()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.Endpoint, buf)
	if err != nil {
		return errorsx.Wrap(err, "unable to create kernel upload request")
	}
	req.Header.Set("Content-Type", mimetype)

	debugx.Println("upload initiated", t.Endpoint)
	resp, err := httpx.AsError(chttp.Do(req)) //nolint:golint,bodyclose
	defer httpx.TryClose(resp)
	debugx.Println("upload completed", t.Endpoint)

	if err != nil {
		return errorsx.Wrap(err, "unable to upload kernel for processing")
	}

	if err = json.NewDecoder(resp.Body).Decode(&e); err != nil {
		return errorsx.Wrap(err, "unable to decode response")
	}

	log.Println("enqueued", spew.Sdump(&e))

	return nil
}
