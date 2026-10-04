package compute

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/internal/unsafepretty"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/egdaemon/eg/secrets"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"
)

type upload struct {
	cmdopts.RuntimeResources
	HostedCompute    bool     `name:"shared-compute" help:"allow hosted compute" default:"true"`
	Direct           bool     `name:"direct" help:"attempt to upload directly to an available runner before falling back to the cluster" default:"true" negatable:""`
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
	Secrets          []string `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
}

// bounds the number of runners we attempt to upload to directly before
// falling back to the cluster.
const directCandidates = 8

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

	if ws, err = workspaces.NewLocal(
		gctx.Context,
		uuid.Must(uuid.NewV7()),
		md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())),
		t.Dir,
		t.Name,
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

	enq := &runners.Enqueued{
		Entry:       filepath.Join(ws.Module, filepath.Base(entry.Path)),
		Ttl:         uint64(t.RuntimeResources.TTL.Milliseconds()),
		Cores:       t.RuntimeResources.Cores,
		Memory:      uint64(t.RuntimeResources.Memory),
		Vram:        uint64(t.RuntimeResources.Vram),
		Arch:        t.RuntimeResources.Arch,
		Os:          t.RuntimeResources.OS,
		AllowShared: t.HostedCompute,
		VcsUri:      errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote)), // optionally set the vcsuri if we're inside a repository.
		VcsCommit:   errorsx.Zero(gitx.Commitish(ws.WorkingDir, t.GitReference)),
		Labels:      append([]string{}, t.RuntimeResources.Labels...),
		Description: t.Name,
	}

	c := tlsc.DefaultClient()
	tokensrc := compute.NewAuthzTokenSource(tlsc.DefaultClient(), signer, authn.EndpointCompute(), gctx.AccountID)
	chttp := oauth2.NewClient(
		context.WithValue(gctx.Context, oauth2.HTTPClient, c),
		tokensrc,
	)

	if t.Direct {
		direct := &runners.EnqueuedDequeueResponse{
			Enqueued: proto.Clone(enq).(*runners.Enqueued),
		}
		direct.Enqueued.Id = uuid.Must(uuid.NewV7()).String()
		direct.Enqueued.AccountId = gctx.AccountID

		if accepted, cause := t.direct(gctx.Context, chttp, direct, archiveio, environio); cause != nil {
			log.Println("direct upload unavailable, falling back to the cluster", cause)
		} else {
			log.Println("enqueued directly", direct.Enqueued.Id, accepted.Id, accepted.P2Pid)
			return nil
		}

		if err = iox.Rewind(archiveio); err != nil {
			return errorsx.Wrap(err, "unable to rewind kernel archive")
		}
	}

	recorded, err := t.enqueue(gctx.Context, chttp, enq, archiveio)
	if err != nil {
		return err
	}

	log.Println("enqueued", spew.Sdump(recorded))
	// TODO: monitoring the job once its uploaded and we have a run id.

	return nil
}

// enqueue uploads the workload to the cluster.
func (t upload) enqueue(ctx context.Context, chttp *http.Client, enq *runners.Enqueued, archive io.Reader) (_ *runners.EnqueuedCreateResponse, err error) {
	var (
		e runners.EnqueuedCreateResponse
	)

	mimetype, buf, err := runners.NewEnqueueUpload(enq, archive)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to generate multipart upload")
	}
	defer buf.Close()

	r := iox.TimeoutReader(10*time.Second, buf)
	defer r.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.Endpoint, r)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to create kernel upload request")
	}
	req.Header.Set("Content-Type", mimetype)

	debugx.Println("upload initiated", t.Endpoint)
	resp, err := httpx.AsError(chttp.Do(req)) //nolint:golint,bodyclose
	defer httpx.TryClose(resp)
	debugx.Println("upload completed", t.Endpoint)

	if err != nil {
		return nil, errorsx.Wrap(err, "unable to upload kernel for processing")
	}

	if err = json.NewDecoder(resp.Body).Decode(&e); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode response")
	}

	if e.Enqueued == nil {
		return nil, errorsx.New("enqueued response missing workload")
	}

	return &e, nil
}

// direct attempts to upload the workload straight to an available runner,
// returning the runner that accepted it. the runner records the workload with
// the cluster using the candidate token issued for it.
func (t upload) direct(ctx context.Context, chttp *http.Client, req *runners.EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (_ *compute.Compute, err error) {
	meta, err := registration.NewPingClient(chttp).Meta(ctx)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve p2p bootstrap addresses")
	}

	p2p, err := libp2px.NewClient(ctx, libp2px.StringsToPeers(meta.Bootstrap...)...)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to join p2p network")
	}
	defer p2p.Close()

	candidates, err := runners.NewPushClient(chttp).Candidates(ctx, req.Enqueued, directCandidates)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve candidate runners")
	}

	if accepted := runners.TryUpload(ctx, p2p, candidates.Items, req, archive, environ); accepted != nil {
		return accepted, nil
	}

	return nil, errorsx.Errorf("no runner accepted the workload, %d candidates", len(candidates.Items))
}
