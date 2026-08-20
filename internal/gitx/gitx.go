package gitx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/object"
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/golang-jwt/jwt/v4"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/egdaemon/eg/internal/tracex"
)

func DetectRoot() string {
	return filepath.Dir(fsx.Locate(".git"))
}

// IsRepository reports whether dir is the root of a git repository.
func IsRepository(dir string) bool {
	dir = filepath.Clean(dir)
	path := filepath.Clean(fsx.LocateWithin(dir, ".git"))
	_, err := filepath.Rel(dir, path)
	if err != nil {
		debugx.Println(errorsx.Wrap(err, "is not a git repository"))
		return false
	}
	return true
}

func Commitish(dir, treeish string) (_ string, err error) {
	var (
		r    *git.Repository
		hash *plumbing.Hash
	)

	if r, err = git.PlainOpen(dir); err != nil {
		return "", errorsx.Wrapf(err, "unable to detect git repository: %s", dir)
	}

	if hash, err = r.ResolveRevision(plumbing.Revision(treeish)); err != nil {
		log.Println("unable to resolve git reference - commit will be empty", dir, treeish, err)
		return "", errorsx.Wrapf(err, "unable to resolve git reference: %s - %s", treeish, dir)
	}

	return hash.String(), nil
}

// Worktree creates a linked worktree at dir, checked out to a detached HEAD
// of repo. gives callers an isolated, real directory backed by repo's own
// object store -- reflecting only committed state, no network clone, and
// without mutating repo's own checkout. dir must not already exist.
//
// go-git has no API to create a linked worktree (only to open one), so this
// shells out to the system git binary.
func Worktree(ctx context.Context, repo, dir string) (err error) {
	if out, perr := gitcmd(ctx, repo, "worktree", "prune").CombinedOutput(); perr != nil {
		log.Println(errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), perr), "unable to prune stale worktrees: %s", repo))
	}

	out, err := gitcmd(ctx, repo, "worktree", "add", "--detach", dir, "HEAD").CombinedOutput()
	if err != nil {
		return errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err), "unable to create worktree: %s -> %s", repo, dir)
	}

	return nil
}

// LocalClone creates a fully self-contained checkout of repo at dir, checked
// out to a detached HEAD, reflecting only committed state. unlike Worktree,
// dir gets its own independent .git directory rather than a .git file
// pointing back into repo's .git/worktrees/<id> -- a linked worktree's gitdir
// reference is an absolute path into repo's own .git, which breaks once dir
// is relocated into a different mount namespace (e.g. bind-mounted into a
// container that doesn't also have repo's .git available at that same path).
// dir must not already exist.
func LocalClone(ctx context.Context, repo, dir string) (err error) {
	origin, err := execx.String(ctx, "git", "-C", repo, "remote", "get-url", git.DefaultRemoteName)
	if err != nil {
		debugx.Println("unable to determine repository origin remote", err)
		// source repo has no origin remote — git clone --local handles this fine,
		// so skip the remote-mutation step rather than failing outright.
		origin = ""
	}
	origin = strings.TrimSpace(origin)

	out, err := execx.RunAs(ctx, eg.DefaultUsername, "git", "clone", "-q", "--local", repo, dir).CombinedOutput()
	if err != nil {
		return errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err), "unable to clone repository: %s -> %s", repo, dir)
	}

	out, err = execx.RunAs(ctx, eg.DefaultUsername, "git", "-C", dir, "checkout", "-q", "--detach", "HEAD").CombinedOutput()
	if err != nil {
		return errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err), "unable to detach HEAD: %s", dir)
	}

	if origin != "" {
		_, err = execx.RunAs(ctx, eg.DefaultUsername, "git", "-C", dir, "remote", "remove", git.DefaultRemoteName).CombinedOutput()
		if err != nil {
			return errorsx.Wrapf(err, "unable to remove origin remote: %s -> %s", repo, dir)
		}

		_, err = execx.RunAs(ctx, eg.DefaultUsername, "git", "-C", dir, "remote", "add", git.DefaultRemoteName, origin).CombinedOutput()
		if err != nil {
			return errorsx.Wrapf(err, "unable to add origin remote: %s -> %s", repo, dir)
		}
	}

	return nil
}

func Clone(ctx context.Context, dir, uri, remote, treeish string, opts ...client.Option) (err error) {
	var (
		r *git.Repository
	)

	branchRefName := plumbing.NewBranchReferenceName(treeish)

	if r, err = git.PlainOpen(dir); err == nil {
		remote, err := r.Remote(remote)
		if err != nil {
			return errorsx.Wrapf(err, "unable to find remote: '%s'", remote)
		}

		if err = remote.FetchContext(ctx, &git.FetchOptions{ClientOptions: opts}); errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil
		} else if err != nil {
			return errorsx.Wrap(err, "unable to fetch")
		}

		w, err := r.Worktree()
		if err != nil {
			return err
		}

		branchCoOpts := git.CheckoutOptions{
			Branch: plumbing.ReferenceName(branchRefName),
			Force:  true,
		}

		return errorsx.Wrapf(w.Checkout(&branchCoOpts), "unable to checkout '%s'", treeish)
	} else {
		log.Println(errorsx.Wrapf(err, "repository is missing attempting clone: %s", uri))
	}

	cloneOpts := &git.CloneOptions{
		URL:               uri,
		ReferenceName:     branchRefName,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		SingleBranch:      true,
		Bare:              false,
		ClientOptions:     opts,
	}

	_, err = git.PlainCloneContext(ctx, dir, cloneOpts)
	if err = errorsx.Wrapf(err, "unable to clone: %s - %s", uri, treeish); err != nil {
		return err
	}

	return nil
}

// return the clone uri handling quirks of specific forges.
// aka: github requires the use of the http clone url for its authentication token.
func QuirkCloneURI(r *git.Repository, name string) (_ string, err error) {
	uri, err := CanonicalURI(r, name)

	replaced := strings.ReplaceAll(uri, "git@github.com:", "https://github.com/")

	return replaced, err
}

// return the canonical URI for a repository according to eg. which is git@host:repository.git
func CanonicalURI(r *git.Repository, name string) (_ string, err error) {
	remote, err := r.Remote(name)
	if err != nil {
		return "", errorsx.Wrapf(err, "unable to detect remote: %s", name)
	}

	return vcsuri(remote.Config().URLs...), nil
}

func Env(repo *git.Repository, remote string, branch string, vcsclone string) (env []string, err error) {
	uri, err := CanonicalURI(repo, remote)
	if err != nil {
		return nil, err
	}

	return HeadEnv(repo, uri, stringsx.First(vcsclone, errorsx.Zero(QuirkCloneURI(repo, remote))), branch)
}

// ideally we shouldn't need this but unfortunately go-git doesn't apply 'instead of' rules properly.
// and from the issue tracker that leads to the question of if it works with the git credential helper.
func LocalEnv(repo *git.Repository, remote string, branch string) (env []string, err error) {
	var (
		benv []string
	)

	uri, err := CanonicalURI(repo, remote)
	if err != nil {
		return nil, err
	}

	if env, err = HeadEnv(repo, uri, eg.DefaultWorkingDirectory(), branch); err != nil {
		return nil, errorsx.Wrapf(err, "head env: %s - %s", uri, branch)
	}

	if benv, err = BaseEnv(repo, uri, eg.DefaultWorkingDirectory(), "main"); err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			tracex.Println("missing base branch for local env - change added for baremetal support in github actions")
			// do nothing, this check was added for baremetal compute.
			// we'll see if its workable.
		} else {
			return nil, errorsx.Wrapf(err, "base env: %s", uri)
		}
	}

	env = append(env, benv...)
	env = append(env, envx.Format(eg.EnvComputeVCS, uri))

	return env, nil
}

func HeadEnv(repo *git.Repository, vcs, uri string, treeish string) (env []string, err error) {
	var (
		hash   *plumbing.Hash
		commit *object.Commit
	)

	if hash, err = repo.ResolveRevision(plumbing.Revision(treeish)); err != nil {
		return nil, errorsx.Wrapf(err, "unable to resolve git revision: %s", treeish)
	}

	if commit, err = repo.CommitObject(*hash); err != nil {
		return nil, errorsx.Wrapf(err, "unable to resolve git reference: %s", treeish)
	}

	return envx.Build().Var(
		eg.EnvGitHeadVCS, vcs,
	).Var(
		eg.EnvGitHeadURI, uri,
	).Var(
		eg.EnvGitHeadRef, treeish,
	).Var(
		eg.EnvGitHeadCommit, commit.Hash.String(),
	).Var(
		eg.EnvGitHeadCommitAuthor, commit.Committer.Name,
	).Var(
		eg.EnvGitHeadCommitEmail, commit.Committer.Email,
	).Var(
		eg.EnvGitHeadCommitTimestamp, commit.Committer.When.Format(time.RFC3339),
	).Environ()
}

func BaseEnv(repo *git.Repository, vcs, uri string, treeish string) (env []string, err error) {
	var (
		hash   *plumbing.Hash
		commit *object.Commit
	)

	if hash, err = repo.ResolveRevision(plumbing.Revision(treeish)); err != nil {
		return nil, errorsx.Wrapf(err, "unable to resolve git revision: %s", treeish)
	}

	if commit, err = repo.CommitObject(*hash); err != nil {
		return nil, errorsx.Wrapf(err, "unable to resolve git reference: %s", treeish)
	}

	return envx.Build().Var(
		eg.EnvGitBaseURI, vcs,
	).Var(
		eg.EnvGitBaseURI, uri,
	).Var(
		eg.EnvGitBaseRef, treeish,
	).Var(
		eg.EnvGitBaseCommit, commit.Hash.String(),
	).Environ()
}

func sshvcsuri(s string) string {
	vcs := errorsx.Zero(url.Parse(s))
	if vcs == nil {
		return s
	}

	vcs.Scheme = "ssh"
	vcs.User = url.User("git")
	return vcs.String()
}

func vcsuri(uris ...string) string {
	uri := slicesx.FirstOrZero(uris...)
	return strings.Replace(strings.TrimPrefix(sshvcsuri(uri), "ssh://"), "/", ":", 1)
}

func VCSDownloadToken(aid string, vcsuri string, options ...jwtx.Option) jwt.RegisteredClaims {
	return jwtx.NewJWTClaims(
		vcsuri,
		jwtx.ClaimsOptionExpiration(24*time.Hour),
		jwtx.ClaimsOptionIssuer(aid),
		jwtx.ClaimsOptionComposed(options...),
	)
}

// Automatically refresh the git credentials from an access token immediately the first time and then periodically in the background.
func AutomaticCredentialRefresh(ctx context.Context, c *http.Client, dst string, token string) error {
	if stringsx.Blank(token) {
		debugx.Println("access token blank skipping")
		return nil
	}

	debugx.Println("periodic git credentials refresh enabled")
	if err := credentialRefresh(ctx, c, dst, token); err != nil {
		return errorsx.Wrap(err, "failed to initially fetch access token")
	}

	go timex.Every(10*time.Minute, func() {
		errorsx.Log(errorsx.Wrap(credentialRefresh(ctx, c, dst, token), "unable to refresh credentials"))
	})

	return nil
}

// RefreshCredentials performs a single, immediate exchange of the given
// long-lived access token for short-lived git credentials, written to
// dst/vcsaccess.token (see LoadCredentials). Unlike AutomaticCredentialRefresh
// it does not spawn a periodic background refresh goroutine, so it's safe to
// call from a short-lived task (e.g. compiling a single workload) without
// leaking a goroutine tied to a long-lived context. A blank token is a no-op
// (matching AutomaticCredentialRefresh) -- callers fall back to LoadCredentials
// finding nothing, and gitx.Clone proceeding with no auth (e.g. public repos).
func RefreshCredentials(ctx context.Context, c *http.Client, dst string, token string) error {
	if stringsx.Blank(token) {
		debugx.Println("access token blank skipping")
		return nil
	}

	return credentialRefresh(ctx, c, dst, token)
}

func credentialRefresh(ctx context.Context, c *http.Client, dst, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/r/vcsaccess/", eg.EnvContainerAPIHostDefault()), nil)
	if err != nil {
		return errorsx.Wrap(err, "unable to create http request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", token))

	resp, err := httpx.AsError(c.Do(req))
	if err != nil {
		return errorsx.Wrap(err, "http request failed")
	}
	defer resp.Body.Close()
	encoded, err := io.ReadAll(resp.Body)
	if err != nil {
		return errorsx.Wrap(err, "unable to create http request")
	}

	if err = os.WriteFile(filepath.Join(dst, "vcsaccess.token"), encoded, 0666); err != nil {
		return errorsx.Wrap(err, "unable to write credentials")
	}

	return nil
}

func LoadCredentials(ctx context.Context, vcsuri string, dir string) (client.Option, error) {
	var (
		httpauth compute.GitCredentialsHTTP
	)
	encoded, err := os.ReadFile(filepath.Join(dir, "vcsaccess.token"))
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(encoded, &httpauth); err == nil && stringsx.Present(httpauth.Username) && stringsx.Present(httpauth.Password) {
		if strings.HasPrefix(vcsuri, "http") {
			return client.WithHTTPAuth(&githttp.BasicAuth{Username: httpauth.Username, Password: httpauth.Password}), nil
		}
	}

	return nil, nil
}

func Bearer(dir string) string {
	var httpauth compute.GitCredentialsHTTP
	encoded, err := os.ReadFile(filepath.Join(dir, "vcsaccess.token"))
	if err == nil {
		if err = json.Unmarshal(encoded, &httpauth); err == nil && stringsx.Present(httpauth.Password) {
			return httpauth.Password
		}
	}

	return ""
}

// gitcmd builds a `git -C repo <args>` invocation, run as the egd user --
// repo is frequently bind-mounted into the workload container and owned by
// the unprivileged egd user rather than whatever uid this process runs as,
// which trips git's "dubious ownership" safe.directory check (CVE-2022-24765
// mitigation) when run directly. see execx.RunAs.
func gitcmd(ctx context.Context, repo string, args ...string) *exec.Cmd {
	return execx.RunAs(ctx, eg.DefaultUsername, "git", append([]string{"-C", repo}, args...)...)
}
