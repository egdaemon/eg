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
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
)

// CompileRequest is the JSON body accepted by POST /c/enqueue and the
// metadata written into the compile spool: a source ref instead of a
// pre-built archive, plus a long-lived signed VcsAuthToken exchanged for
// short-lived git credentials immediately before cloning (see
// gitx.RefreshCredentials) rather than trusted as-is for the lifetime of the
// job -- it is never written back to disk once the clone is done.
type CompileRequest struct {
	AccountId    string   `json:"account_id"`
	ClusterId    string   `json:"cluster_id"`
	VcsUri       string   `json:"vcs_uri"`
	VcsRef       string   `json:"vcs_ref"`
	VcsCommit    string   `json:"vcs_commit"`
	VcsAuthToken string   `json:"vcs_auth_token"`
	Entry        string   `json:"entry"`
	Cores        uint64   `json:"cores"`
	Memory       uint64   `json:"memory"`
	Vram         uint64   `json:"vram"`
	Ttl          uint64   `json:"ttl"`
	Labels       []string `json:"labels"`
}

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

// compileWorkload reads the CompileRequest stored at dir/metadata.json
// (dir is a job claimed from a compile SpoolDirs' Running stage, see
// CompileN), clones and compiles the referenced source, then hands the
// entire job directory off directly to rundirs.Queued -- bypassing
// rundirs.Downloading/Enqueue entirely, since the job is already fully
// formed once compiled and doesn't need to be staged through a second
// spool's own two-step download/enqueue handshake.
func compileWorkload(ctx context.Context, c *http.Client, dir string, rundirs SpoolDirs) (err error) {
	var (
		encoded []byte
		req     CompileRequest
		repo    *git.Repository
	)

	if encoded, err = os.ReadFile(filepath.Join(dir, "metadata.json")); err != nil {
		return errorsx.Wrap(err, "unable to read compile request")
	}

	if err = json.Unmarshal(encoded, &req); err != nil {
		return errorsx.Wrap(err, "unable to decode compile request")
	}

	treeish := stringsx.DefaultIfBlank(req.VcsCommit, req.VcsRef)
	clonedir := filepath.Join(dir, "src")

	if err = os.MkdirAll(clonedir, 0700); err != nil {
		return errorsx.Wrap(err, "unable to create clone directory")
	}

	// exchange the long-lived request token for short-lived git credentials
	// immediately before cloning, rather than trusting whatever was handed to
	// the enqueue endpoint -- avoids needing to track/check token expiry
	// ourselves while the job sat queued behind the compile concurrency cap.
	if err = gitx.RefreshCredentials(ctx, c, clonedir, req.VcsAuthToken); err != nil {
		return errorsx.Wrap(err, "unable to refresh git credentials")
	}

	// mirrors interp/runtime/wasi/ffigit.go's autoauth: absence of refreshed
	// credentials (e.g. blank VcsAuthToken, public repo) is not fatal -- log
	// and fall back to an unauthenticated clone rather than failing the job.
	auth, err := gitx.LoadCredentials(ctx, req.VcsUri, clonedir)
	if err != nil {
		log.Println(errorsx.Wrap(err, "unable to load git credentials"))
	}

	if err = gitx.Clone(ctx, auth, clonedir, req.VcsUri, treeish, treeish); err != nil {
		return errorsx.Wrap(err, "unable to clone repository")
	}

	if repo, err = git.PlainOpen(clonedir); err != nil {
		return errorsx.Wrap(err, "unable to open cloned repository")
	}

	ws, err := workspaces.New(ctx, md5.New(), clonedir, clonedir, req.Entry)
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

	envb := envx.Build().FromEnviron(errorsx.Zero(gitx.HeadEnv(repo, req.VcsUri, req.VcsUri, treeish))...)

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

	uid := Queued().Id(dir)
	response := EnqueuedDequeueResponse{
		Enqueued: &Enqueued{
			Id:        uid.String(),
			AccountId: req.AccountId,
			ClusterId: req.ClusterId,
			VcsUri:    req.VcsUri,
			Entry:     entry,
			Cores:     req.Cores,
			Memory:    req.Memory,
			Vram:      req.Vram,
			Ttl:       req.Ttl,
			Labels:    req.Labels,
		},
	}

	if encoded, err = json.Marshal(&response); err != nil {
		return errorsx.Wrap(err, "unable to encode workload metadata")
	}

	// overwrite in place: this drops VcsAuthToken from disk now that the clone is done.
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
