package compute

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

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
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
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

type upload struct {
	cmdopts.RuntimeResources
	HostedCompute bool     `name:"shared-compute" help:"allow hosted compute" default:"true"`
	SSHKeyPath    string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Dir           string   `name:"directory" help:"root directory of the repository" default:"${vars_git_directory}"`
	ModuleDir     string   `name:"moduledir" help:"must be a subdirectory in the provided directory" default:"${vars_workload_directory}"`
	Name          string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload"`
	Environment   []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Dirty         bool     `name:"dirty" help:"include all environment variables"`
	Endpoint      string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
	GitRemote     string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference  string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	GitClone      string   `name:"git-clone-uri" help:"clone uri"`
}

func (t upload) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
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

	if ws, err = workspaces.New(gctx.Context, t.Dir, t.ModuleDir, t.Name, false); err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Join(ws.Root, ws.RuntimeDir))

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(gctx.Context)
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

	if repo, err = git.PlainOpen(ws.Root); err != nil {
		return errorsx.Wrap(err, "unable to open git repository")
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
	// if err = tarx.Inspect(archiveio); err != nil {
	// 	log.Println(errorsx.Wrap(err, "unable to inspect archive"))
	// }

	// if err = iox.Rewind(archiveio); err != nil {
	// 	return errorsx.Wrap(err, "unable to rewind kernel archive")
	// }

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

	c := httpx.BindRetryTransport(tlsc.DefaultClient(), http.StatusTooManyRequests, http.StatusBadGateway)
	tokensrc := compute.NewAuthzTokenSource(c, signer, authn.EndpointCompute())
	chttp := oauth2.NewClient(
		context.WithValue(gctx.Context, oauth2.HTTPClient, c),
		tokensrc,
	)

	req, err := http.NewRequestWithContext(gctx.Context, http.MethodPost, t.Endpoint, buf)
	if err != nil {
		return errorsx.Wrap(err, "unable to create kernel upload request")
	}
	req.Header.Set("Content-Type", mimetype)

	log.Println("upload initiated", t.Endpoint)
	resp, err := httpx.AsError(chttp.Do(req)) //nolint:golint,bodyclose
	defer httpx.TryClose(resp)
	defer runtime.KeepAlive(resp) // guess
	log.Println("upload completed", t.Endpoint)

	if err != nil {
		return errorsx.Wrap(err, "unable to upload kernel for processing")
	}

	// TODO: monitoring the job once its uploaded and we have a run id.

	return nil
}
