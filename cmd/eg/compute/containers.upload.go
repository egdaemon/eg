package compute

import (
	"context"
	"embed"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

// eg local
// eg compute upload // current upload...
// eg compute c8s upload path/to/Containerfile

type c8scmds struct {
	Upload c8sUpload `cmd:"" help:"upload and run a container file"`
	Local  c8sLocal  `cmd:"" help:"upload and run a container file"`
}

//go:embed .bootstrap.c8s
var embeddedc8supload embed.FS

type c8sUpload struct {
	runtimecfg    cmdopts.RuntimeResources
	HostedCompute bool     `name:"shared-compute" help:"allow hosted compute" default:"true"`
	Containerfile string   `arg:"" help:"path to the container file to run" default:"Containerfile"`
	SSHKeyPath    string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Environment   []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote     string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	Endpoint      string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/manager/" hidden:"true"`
}

func (t c8sUpload) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
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

	if repo, err = git.PlainOpen(ws.Root); err != nil {
		return errorsx.Wrap(err, "unable to open git repository")
	}

	if tmpdir, err = os.MkdirTemp("", "eg.c8s.upload.*"); err != nil {
		return errorsx.Wrap(err, "unable to  create temporary directory")
	}

	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	egdir := filepath.Join(tmpdir, ".eg")
	if err = fsx.MkDirs(0700, egdir, filepath.Join(tmpdir, buildir, "mounted", "workspace")); err != nil {
		return err
	}

	autoruncontainer := filepath.Join(tmpdir, buildir, "mounted", "workspace", "Containerfile")

	if err = fsx.CloneTree(gctx.Context, egdir, ".bootstrap.c8s", embeddedc8supload); err != nil {
		return err
	}

	if err = compile.InitGolang(gctx.Context, egdir, cmdopts.ModPath()); err != nil {
		return err
	}

	if err = iox.Copy(t.Containerfile, autoruncontainer); err != nil {
		return err
	}

	if ws, err = workspaces.New(gctx.Context, tmpdir, ".eg", ""); err != nil {
		return err
	}

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(gctx.Context)
	if err != nil {
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

	if environio, err = os.Create(filepath.Join(tmpdir, "environ.env")); err != nil {
		return errorsx.Wrap(err, "unable to open the kernel archive")
	}
	defer environio.Close()

	envb := envx.Build().
		FromEnviron(t.Environment...)

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to write environment variables buffer")
	}

	if err = iox.Rewind(environio); err != nil {
		return errorsx.Wrap(err, "unable to rewind environment variables buffer")
	}

	// debugx.Println(envx.PrintEnv(errorsx.Zero(envb.Environ())...))

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

	// if err = iox.Copy(archiveio.Name(), "archive.tar.gz"); err != nil {
	// 	return errorsx.Wrap(err, "unable to copy archive")
	// }
	ainfo := errorsx.Zero(os.Stat(archiveio.Name()))
	log.Println("archive metadata", ainfo.Name(), ainfo.Size())

	// TODO: determine the destination based on the requirements
	// i.e. cores, memory, labels, disk, videomem, etc.
	// not sure if the client should do this or the node we upload to.
	// if its the node we upload to it'll cost more due to having to
	// push the archive to another node that matches the requirements.
	// in theory we could use redirects to handle that but it'd still take a performance hit.
	mimetype, buf, err := runners.NewEnqueueUpload(&runners.Enqueued{
		AllowShared: t.HostedCompute,
		Entry:       filepath.Base(entry.Path),
		Ttl:         uint64(t.runtimecfg.TTL.Milliseconds()),
		Cores:       t.runtimecfg.Cores,
		Memory:      t.runtimecfg.Memory,
		Arch:        t.runtimecfg.Arch,
		Os:          t.runtimecfg.OS,
		Vcsuri:      errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)), // optionally set the vcsuri if we're inside a repository.

	}, archiveio)
	if err != nil {
		return errorsx.Wrap(err, "unable to generate multipart upload")
	}

	chttp, err := authn.OAuth2SSHHTTPClient(
		context.WithValue(gctx.Context, oauth2.HTTPClient, tlsc.DefaultClient()),
		signer,
		authn.EndpointSSHAuth(),
	)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(gctx.Context, http.MethodPost, t.Endpoint, buf)
	if err != nil {
		return errorsx.Wrap(err, "unable to create kernel upload request")
	}
	req.Header.Set("Content-Type", mimetype)

	resp, err := httpx.AsError(chttp.Do(req)) //nolint:golint,bodyclose
	defer httpx.TryClose(resp)

	if err != nil {
		return errorsx.Wrap(err, "unable to upload kernel for processing")
	}

	return nil
}
