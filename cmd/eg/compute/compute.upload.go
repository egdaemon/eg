package compute

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"

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
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/internal/unsafepretty"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/secrets"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/ssh"
)

type upload struct {
	cmdopts.RuntimeResources
	HostedCompute    bool          `name:"shared-compute" help:"allow hosted compute" default:"true"`
	P2P              int           `name:"p2p" help:"priority of uploading directly to a runner the control plane suggests before falling back to the cluster, higher goes first, 0 disables" default:"1" env:"EG_P2P_PRIORITY"`
	LAN              int           `name:"lan" help:"priority of uploading directly to a runner remembered from the local network (see 'eg compute lan'), higher goes first, 0 disables including scanning for runners" default:"2" env:"EG_LAN_PRIORITY"`
	P2POnly          bool          `name:"p2p-only" help:"never fall back to uploading to the cluster, fail if no runner accepts a direct upload" env:"EG_P2P_ONLY"`
	CacheDir         string        `name:"cache-directory" help:"directory the remembered systems on the local network are stored in" default:"${vars_cache_directory}" env:"EG_CACHE_DIRECTORY"`
	LANScanTimeout   time.Duration `name:"lan-scan-timeout" help:"how long the background scan of the local network runs for" default:"3s"`
	LANPeerTTL       time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems on the local network are remembered after they were last seen" default:"720h" env:"EG_LAN_PEER_TTL"`
	Follow           bool          `name:"follow" help:"stream the workload's logs once it is running, requires a runner supporting log streaming" negatable:"" default:"true"`
	SSHKeyPath       string        `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Dir              string        `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	Name             string        `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload"`
	EnvironmentPaths []string      `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string      `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Dirty            bool          `name:"dirty" help:"include all environment variables"`
	Endpoint         string        `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
	GitRemote        string        `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string        `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	GitClone         string        `name:"git-clone-uri" help:"clone uri"`
	GCPAuto          bool          `name:"gcp-auto" help:"have the runner issue the account's gcp credentials (workload identity) to the workload"`
	Secrets          []string      `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
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
		lancache             *sql.DB
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	// remember the runners on the local network while the workload is being built, the scan only
	// feeds the cache so it never delays the upload. the cache is optional.
	if t.LAN > 0 {
		if lancache, err = lan.Open(gctx.Context, filepath.Join(t.CacheDir, migrations.CLI), t.LANPeerTTL); err != nil {
			log.Println(errorsx.Wrap(err, "the local network cache is unavailable"))
			lancache = nil
		} else {
			defer lancache.Close()
			defer autoscan(gctx.Context, lancache, find, t.LANScanTimeout)()
		}
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

	enq := &runners.Enqueued{
		Id:          uuid.Must(uuid.NewV7()).String(),
		AccountId:   aid,
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
		Description: stringsx.FirstNonBlank(t.Name, gitx.Description("Uploaded: %git.commit.author% - %git.ref%", repo, t.GitReference)),
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource)

	if order := priorities(t.LAN, t.P2P); len(order) > 0 {
		// a nil runner means nothing accepted the workload. otherwise it was uploaded and
		// the error, if any, is the result of following it, so we must not upload it again.
		accepted, cause := t.p2p(gctx.Context, chttp, lancache, order, enq, archiveio, environio)
		if accepted != nil {
			log.Println("enqueued directly", enq.Id, accepted.Id, accepted.P2Pid)
			return cause
		}

		if t.P2POnly {
			return errorsx.Wrap(cause, "direct upload unavailable and --p2p-only is set")
		}

		log.Println("direct upload unavailable, falling back to the cluster", cause)

		if err = iox.Rewind(archiveio); err != nil {
			return errorsx.Wrap(err, "unable to rewind kernel archive")
		}
	} else if t.P2POnly {
		return errorsx.Errorf("--p2p-only is set but no direct upload strategy is enabled, see --p2p and --lan")
	}

	recorded, err := t.enqueue(gctx.Context, chttp, enq, archiveio)
	if err != nil {
		return err
	}

	log.Println("enqueued", spew.Sdump(recorded))

	return t.followenqueued(gctx.Context, chttp, recorded.Enqueued.Id)
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
