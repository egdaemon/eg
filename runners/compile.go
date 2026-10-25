package runners

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/client"
)

// compileEntrypoint transpiles and builds the eg module rooted at ws,
// returning the (non-generated) compiled entrypoint. Mirrors
// egmeta/daemons/ci/compute.Compile, using only eg-owned packages so it
// no longer needs to round-trip through egmeta.
func compileEntrypoint(ctx context.Context, ws workspaces.Context) (*transpile.Compiled, error) {
	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(ws.Root), ws)).Run(ctx)
	if err != nil {
		return nil, err
	}

	if err = compile.EnsureRequiredPackages(ctx, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return nil, err
	}

	modules, err := compile.FromTranspiled(ctx, ws, roots...)
	if err != nil {
		return nil, err
	}

	entry, found := slicesx.Find(func(c transpile.Compiled) bool {
		return !c.Generated
	}, modules...)
	if !found {
		return nil, errors.New("unable to locate entry point")
	}

	return &entry, nil
}

// compileWorkload reads the computeapi.EnqueuedDequeueResponse stored at
// dir/metadata.json (dir is a job claimed from a compile SpoolDirs' Running
// stage, see CompileN) -- AccessToken is exchanged for short-lived git
// credentials, mirroring how egmeta's own /c/dequeue handler populates it for
// the polling flow (see computeapi/http.queue.go's dequeue handler) -- clones
// the uri/ref named by the submitted environ (see eg.EnvironFile) and
// compiles it, then enqueues the compiled workload into rundirs and removes
// the compile job directory.
func compileWorkload(ctx context.Context, c *http.Client, dir string, rundirs SpoolDirs) (err error) {
	return compileWorkloadReserved(ctx, c, httpx.NewFixedStatusClient(http.StatusNotImplemented), dir, rundirs)
}

// compileWorkloadReserved behaves as compileWorkload. c exchanges access tokens
// for git credentials; authclient is the runner's authenticated client used to
// reserve workloads pushed to this runner (those carrying a candidate token, see
// CandidateTokenFile) with the control plane and report their progress.
func compileWorkloadReserved(ctx context.Context, c *http.Client, authclient *http.Client, dir string, rundirs SpoolDirs) (err error) {
	md := compilemetadata{
		c:          c,
		authclient: authclient,
		rundirs:    rundirs,
		done: func(_ string, cause error) state {
			err = cause
			return nil
		},
	}

	for s := compilebegin(md, dir); s != nil; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			s = s.Update(ctx)
		}
	}

	return err
}

type compilemetadata struct {
	c          *http.Client
	authclient *http.Client
	rundirs    SpoolDirs
	// done transitions out of a workload once it has been handed off (cause is
	// nil) or failed.
	done func(dir string, cause error) state
}

// compilejob is a workload claimed from the compile spool.
type compilejob struct {
	compilemetadata
	dir   string
	token string
	req   *computeapi.EnqueuedDequeueResponse
}

func compilebegin(md compilemetadata, dir string) state {
	var (
		err     error
		encoded []byte
		req     computeapi.EnqueuedDequeueResponse
	)

	log.Println("compiling workload initiated", dir)

	if encoded, err = os.ReadFile(filepath.Join(dir, "metadata.json")); err != nil {
		return md.done(dir, errorsx.Wrap(err, "unable to read compile request"))
	}

	if err = json.Unmarshal(encoded, &req); err != nil {
		return md.done(dir, errorsx.Wrap(err, "unable to decode compile request"))
	}

	if req.Enqueued == nil {
		return md.done(dir, errors.New("invalid workload missing enqueued information"))
	}

	job := compilejob{compilemetadata: md, dir: dir, token: ReadCandidateToken(dir), req: &req}

	return status(md.authclient, job.token, StatusCompiling, req.Enqueued, statecompiling{compilejob: job})
}

func compilefailed(job compilejob, cause error) state {
	return status(job.authclient, job.token, StatusFailed, job.req.Enqueued, job.done(job.dir, cause))
}

// statecompiling clones and compiles the workload's source into its archive.
type statecompiling struct {
	compilejob
}

func (t statecompiling) Update(ctx context.Context) state {
	var (
		err  error
		auth client.Option
		repo *git.Repository
	)

	clonedir := filepath.Join(t.dir, "src")
	environpath := filepath.Join(t.dir, eg.EnvironFile)

	// clone from the uri and ref submitted with the workload, the same ones
	// the workload itself clones from (see eggit.AutoClone).
	submitted, err := envx.Build().FromPath(environpath).Environ()
	if err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to read submitted environment"))
	}

	environ := envx.NewEnvironFromStrings(submitted...)
	uri, ref := environ.String("", eg.EnvGitHeadURI), environ.String("", eg.EnvGitHeadRef)
	if stringsx.Blank(uri) || stringsx.Blank(ref) {
		return compilefailed(t.compilejob, errorsx.Errorf("submitted environment missing %s or %s", eg.EnvGitHeadURI, eg.EnvGitHeadRef))
	}

	if err = os.MkdirAll(clonedir, 0700); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to create clone directory"))
	}

	// exchange the access token for short-lived git credentials immediately
	// before cloning, rather than trusting it for the lifetime of the job --
	// avoids needing to track/check token expiry ourselves while the job sat
	// queued behind the compile concurrency cap.
	if auth, err = gitx.ExchangeCredentials(ctx, t.c, uri, t.req.AccessToken); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to exchange git credentials"))
	}

	var opts []client.Option
	if auth != nil {
		opts = append(opts, auth)
	}

	if err = gitx.Clone(ctx, clonedir, uri, git.DefaultRemoteName, ref, opts...); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to clone repository"))
	}

	if repo, err = git.PlainOpen(clonedir); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to open cloned repository"))
	}

	ws, err := workspaces.New(ctx, md5.New(), clonedir, clonedir, t.req.Enqueued.Entry)
	if err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to create workspace"))
	}

	module, err := compileEntrypoint(ctx, ws)
	if err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to compile module"))
	}

	entry, err := filepath.Rel(filepath.Join(ws.Root, ws.BuildDir), module.Path)
	if err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to determine entry relative path"))
	}

	// the environment submitted with the workload takes precedence over what
	// can be determined from the commit alone (e.g. the branch being built).
	envb := envx.Build().
		FromEnviron(errorsx.Zero(gitx.HeadEnv(repo, t.req.Enqueued.VcsUri, t.req.Enqueued.VcsUri, t.req.Enqueued.VcsCommit))...).
		FromPath(environpath)
	environio, err := os.Create(environpath)
	if err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to create environment file"))
	}
	defer environio.Close()

	if err = envb.CopyTo(environio); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to write environment variables"))
	}

	archiveio, err := os.Create(filepath.Join(t.dir, "archive.tar.gz"))
	if err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to create archive"))
	}
	defer archiveio.Close()

	if err = tarx.Pack(archiveio, filepath.Join(ws.Root, ws.BuildDir), environpath); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to pack archive"))
	}

	t.req.Enqueued.Id = Queued().Id(t.dir).String()
	t.req.Enqueued.Entry = entry

	if t.token != "" {
		return statereserve{compilejob: t.compilejob}
	}

	return statehandoff{compilejob: t.compilejob}
}

// statereserve records the compiled workload with the control plane, claimed
// by this runner; it is recorded under the id the control plane pushed it with.
type statereserve struct {
	compilejob
}

func (t statereserve) Update(ctx context.Context) state {
	var (
		recorded *computeapi.EnqueuedDequeueResponse
	)

	archiveio, err := os.Open(filepath.Join(t.dir, "archive.tar.gz"))
	if err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to open archive"))
	}
	defer archiveio.Close()

	if recorded, err = NewDirectClient(t.authclient).Reserve(ctx, t.req.Enqueued, archiveio, t.token); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to reserve workload"))
	}

	t.req.Enqueued, t.req.AccessToken = recorded.Enqueued, recorded.AccessToken

	return statehandoff{compilejob: t.compilejob}
}

// statehandoff enqueues the compiled workload into rundirs, then removes the
// compile job directory entirely.
type statehandoff struct {
	compilejob
}

func (t statehandoff) Update(ctx context.Context) state {
	uid := Queued().Id(t.dir)

	// metadata.json now holds the compiled workload's entry point and id,
	// still carrying AccessToken through for the running container (see
	// queue.go's beginwork).
	encoded, err := json.Marshal(t.req)
	if err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to encode workload metadata"))
	}

	if err = t.rundirs.Download(uid, "metadata.json", bytes.NewReader(encoded)); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to write workload metadata"))
	}

	archiveio, err := os.Open(filepath.Join(t.dir, "archive.tar.gz"))
	if err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to open archive"))
	}
	defer archiveio.Close()

	if err = t.rundirs.Download(uid, "archive.tar.gz", archiveio); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to write archive"))
	}

	if t.token != "" {
		if err = t.rundirs.Download(uid, CandidateTokenFile, strings.NewReader(t.token)); err != nil {
			return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to write candidate token"))
		}
	}

	if err = t.rundirs.Enqueue(uid); err != nil {
		return compilefailed(t.compilejob, errorsx.Wrap(err, "unable to hand off compiled workload"))
	}

	log.Println("compiling workload completed", t.dir)

	return status(t.authclient, t.token, StatusQueued, t.req.Enqueued, t.done(t.dir, nil))
}

// compilediscard clears a completed compile job from the compile spool,
// whether it failed (cause is non-nil) or was handed off.
type compilediscard struct {
	compiledirs SpoolDirs
	dir         string
	cause       error
	next        state
}

func (t compilediscard) Update(ctx context.Context) state {
	errorsx.Log(errorsx.Wrap(t.cause, "compile failed"))
	errorsx.Log(errorsx.Wrap(t.compiledirs.Discard(t.dir), "failed to clear compile job"))
	return t.next
}
