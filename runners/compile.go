package runners

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/slicesx"
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

// compileWorkload reads the EnqueuedDequeueResponse stored at
// dir/metadata.json (dir is a job claimed from a compile SpoolDirs' Running
// stage, see CompileN) -- Enqueued.VcsCommit is the treeish to check out,
// AccessToken is exchanged for short-lived git credentials, mirroring how
// egmeta's own /c/dequeue handler populates it for the polling flow (see
// computeapi/http.queue.go's dequeue handler) -- clones and compiles the
// referenced source, then hands the entire job directory off directly to
// rundirs.Queued -- bypassing rundirs.Downloading/Enqueue entirely, since
// the job is already fully formed once compiled and doesn't need to be
// staged through a second spool's own two-step download/enqueue handshake.
func compileWorkload(ctx context.Context, c *http.Client, dir string, rundirs SpoolDirs) (err error) {
	var (
		auth    client.Option
		encoded []byte
		req     EnqueuedDequeueResponse
		repo    *git.Repository
	)

	if encoded, err = os.ReadFile(filepath.Join(dir, "metadata.json")); err != nil {
		return errorsx.Wrap(err, "unable to read compile request")
	}

	if err = json.Unmarshal(encoded, &req); err != nil {
		return errorsx.Wrap(err, "unable to decode compile request")
	}

	if req.Enqueued == nil {
		return errorsx.Wrap(err, "invalid workload missing enqueued information")
	}

	clonedir := filepath.Join(dir, "src")

	if err = os.MkdirAll(clonedir, 0700); err != nil {
		return errorsx.Wrap(err, "unable to create clone directory")
	}

	// exchange the access token (if any) for short-lived git credentials
	// immediately before cloning, rather than trusting it for the lifetime of
	// the job -- avoids needing to track/check token expiry ourselves while
	// the job sat queued behind the compile concurrency cap.
	if err = gitx.RefreshCredentials(ctx, c, clonedir, req.AccessToken); err != nil {
		return errorsx.Wrap(err, "unable to refresh git credentials")
	}

	// absence of refreshed credentials (e.g. blank access token, public
	// repo) is not fatal -- log and fall through to an unauthenticated clone
	// rather than failing the job.
	var opts []client.Option
	if auth, err = gitx.LoadCredentials(ctx, req.Enqueued.VcsUri, clonedir); err != nil {
		log.Println(errorsx.Wrap(err, "unable to load git credentials"))
	} else if auth != nil {
		opts = append(opts, auth)
	}

	if err = gitx.Clone(ctx, clonedir, req.Enqueued.VcsUri, git.DefaultRemoteName, req.Enqueued.VcsCommit, opts...); err != nil {
		return errorsx.Wrap(err, "unable to clone repository")
	}

	if repo, err = git.PlainOpen(clonedir); err != nil {
		return errorsx.Wrap(err, "unable to open cloned repository")
	}

	ws, err := workspaces.New(ctx, md5.New(), clonedir, clonedir, req.Enqueued.Entry)
	if err != nil {
		return errorsx.Wrap(err, "unable to create workspace")
	}

	module, err := compileEntrypoint(ctx, ws)
	if err != nil {
		return errorsx.Wrap(err, "unable to compile module")
	}

	entry, err := filepath.Rel(filepath.Join(ws.Root, ws.BuildDir), module.Path)
	if err != nil {
		return errorsx.Wrap(err, "unable to determine entry relative path")
	}

	envb := envx.Build().FromEnviron(errorsx.Zero(gitx.HeadEnv(repo, req.Enqueued.VcsUri, req.Enqueued.VcsUri, req.Enqueued.VcsCommit))...)

	environpath := filepath.Join(dir, eg.EnvironFile)
	environio, err := os.Create(environpath)
	if err != nil {
		return errorsx.Wrap(err, "unable to create environment file")
	}
	defer environio.Close()

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to write environment variables")
	}

	archiveio, err := os.Create(filepath.Join(dir, "archive.tar.gz"))
	if err != nil {
		return errorsx.Wrap(err, "unable to create archive")
	}
	defer archiveio.Close()

	if err = tarx.Pack(archiveio, filepath.Join(ws.Root, ws.BuildDir), environpath); err != nil {
		return errorsx.Wrap(err, "unable to pack archive")
	}

	req.Enqueued.Id = Queued().Id(dir).String()
	req.Enqueued.Entry = entry

	// overwrite in place: metadata.json now holds the compiled workload's
	// entry point and id, still carrying AccessToken through for the running
	// container (see queue.go's beginwork).
	if encoded, err = json.Marshal(&req); err != nil {
		return errorsx.Wrap(err, "unable to encode workload metadata")
	}

	if err = os.WriteFile(filepath.Join(dir, "metadata.json"), encoded, 0600); err != nil {
		return errorsx.Wrap(err, "unable to overwrite workload metadata")
	}

	// clone artifacts have already been compiled into the archive above; drop
	// them before handing the directory off so they don't linger in rundirs.
	if err = os.RemoveAll(clonedir); err != nil {
		return errorsx.Wrap(err, "unable to clean up clone directory")
	}

	target := filepath.Join(rundirs.Queued, filepath.Base(dir))
	return errorsx.Wrap(os.Rename(dir, target), "unable to hand off compiled workload")
}
