package gitx

import (
	"bytes"
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
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/object"
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"

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
	"github.com/egdaemon/eg/internal/userx"
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

	out, err := gitcmd(ctx, repo, "worktree", "add", "--detach", dir, plumbing.HEAD.String()).CombinedOutput()
	if err != nil {
		return errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err), "unable to create worktree: %s -> %s", repo, dir)
	}

	return nil
}

// LocalClone creates a fully self-contained checkout of repo at dir, checked
// out to a detached HEAD at headtreeish (HEAD when blank), reflecting only
// committed state. unlike Worktree, dir gets its own independent .git directory
// rather than a .git file pointing back into repo's .git/worktrees/<id> -- a
// linked worktree's gitdir reference is an absolute path into repo's own .git,
// which breaks once dir is relocated into a different mount namespace (e.g.
// bind-mounted into a container that doesn't also have repo's .git available at
// that same path). dir must not already exist.
//
// the objects are hardlinked from repo, and copied when that isn't possible
// (repo and dir on different filesystems), so the history is complete.
//
// the clone has the branches of repo as branches, and the remote-tracking
// branches of repo as its own, so they can all be resolved within the checkout.
// when basetreeish is a branch of repo (or of its origin remote) it also exists
// as a branch of the clone, and is recorded as its init.defaultBranch so the
// clone knows its own base, see DefaultBase. a base that is blank or isn't a
// branch is not recorded, any revision of repo resolves in the clone anyway.
func LocalClone(ctx context.Context, repo, dir, basetreeish, headtreeish string) (err error) {
	username := RepositoryUsername(repo)

	run := func(args ...string) (string, error) {
		out, err := execx.RunAs(ctx, username, "git", args...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
		}

		return strings.TrimSpace(string(out)), nil
	}

	// the commit a revision of repo names, revisions can't be mistaken for options.
	resolve := func(treeish string) (string, error) {
		if treeish == "" || strings.HasPrefix(treeish, "-") {
			return "", fmt.Errorf("invalid revision: %q", treeish)
		}

		return run("-C", repo, "rev-parse", "-q", "--verify", treeish+"^{commit}")
	}

	origin, err := execx.String(ctx, "git", "-C", repo, "remote", "get-url", git.DefaultRemoteName)
	if err != nil {
		debugx.Println("unable to determine repository origin remote", err)
		// source repo has no origin remote — git clone --local handles this fine,
		// so skip the remote-mutation step rather than failing outright.
		origin = ""
	}
	origin = strings.TrimSpace(origin)

	headtreeish = stringsx.FirstNonBlank(headtreeish, plumbing.HEAD.String())
	head, err := resolve(headtreeish)
	if err != nil {
		return errorsx.Wrapf(err, "unable to resolve %s: %s", headtreeish, repo)
	}

	// the base when it is a branch, its tip as the local branch and as the remote-tracking branch, either may be missing.
	var basename string
	localbase, _ := resolve(plumbing.NewBranchReferenceName(basetreeish).String())
	remotebase, _ := resolve(plumbing.NewRemoteReferenceName(git.DefaultRemoteName, basetreeish).String())
	switch {
	case stringsx.First(localbase, remotebase) != "":
		basename = basetreeish
	case basetreeish != "":
		debugx.Println("the base isn't a branch, it isn't recorded", basetreeish)
	}

	out, err := execx.RunAs(ctx, username, "git", "clone", "-q", "--local", repo, dir).CombinedOutput()
	if err != nil {
		// hardlinks can't cross filesystems, copy the objects instead. this copies the whole history, which is
		// the one place copying less would pay off. it can be done by not cloning at all: git init dir, then
		// git fetch --depth=<rev-list --count base..head> file://<repo> +<head>:<a scratch ref>, and the same with
		// --depth=1 for the tip of the base, delete the scratch ref, create the branches and the init.defaultBranch
		// record as below, and check out head. fetching into a ref is what also brings the tags of the carried
		// commits. it was measured at 4.7 MiB against 24.8 MiB for a repository with 31 commits of history, and it
		// works when cloning from a clone, which --shallow-exclude does not. not done because the workspace would
		// then differ with the filesystem: the history is cut, only those tags exist, and the other branches of
		// the source and any --git-ref other than head and the base can't be resolved.
		debugx.Println("unable to hardlink the repository, copying", err, strings.TrimSpace(string(out)))
		out, err = execx.RunAs(ctx, username, "git", "clone", "-q", "--local", "--no-hardlinks", repo, dir).CombinedOutput()
		if err != nil {
			return errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err), "unable to clone repository: %s -> %s", repo, dir)
		}
	}

	if _, err = run("-C", dir, "checkout", "-q", "--detach", head); err != nil {
		return errorsx.Wrapf(err, "unable to detach HEAD: %s", dir)
	}

	// a clone sees the branches of the source as remote-tracking ones and only creates a local branch for the
	// one checked out. mirror the refs of the source under their own names instead, the objects are all present
	// so nothing is transferred. pruning drops what the source doesn't have, the clone's mirrored origin/*. its
	// remote HEAD is dropped, what the clone considers its base is recorded explicitly below.
	_, _ = run("-C", dir, "symbolic-ref", "--delete", plumbing.NewRemoteHEADReferenceName(git.DefaultRemoteName).String())
	abs, err := filepath.Abs(repo)
	if err != nil {
		return errorsx.Wrapf(err, "unable to resolve repository path: %s", repo)
	}

	branches := plumbing.NewBranchReferenceName("*").String()
	tracked := plumbing.NewRemoteReferenceName(git.DefaultRemoteName, "*").String()
	if _, err = run("-C", dir, "fetch", "-q", "--no-tags", "--prune", "file://"+abs, "+"+branches+":"+branches, "+"+tracked+":"+tracked, "^"+plumbing.NewRemoteHEADReferenceName(git.DefaultRemoteName).String()); err != nil {
		return errorsx.Wrapf(err, "unable to carry the branches: %s -> %s", repo, dir)
	}

	if basename != "" {
		// the branch exists locally even if repo only had it as a remote-tracking branch.
		if localbase == "" {
			if _, err = run("-C", dir, "update-ref", plumbing.NewBranchReferenceName(basename).String(), remotebase); err != nil {
				return errorsx.Wrapf(err, "unable to create branch %s: %s", basename, dir)
			}
		}

		if _, err = run("-C", dir, "config", "init.defaultBranch", basename); err != nil {
			return errorsx.Wrapf(err, "unable to record the base: %s", dir)
		}
	}

	if origin != "" {
		if _, err = run("-C", dir, "remote", "set-url", git.DefaultRemoteName, origin); err != nil {
			return errorsx.Wrapf(err, "unable to set the origin remote: %s -> %s", repo, dir)
		}
	}

	return nil
}

// DefaultBase finds the base branch of repo, the one workloads are compared against, and the commit it is at.
// the repository is asked first: the branch the remote's HEAD points at, then init.defaultBranch, then go-git's
// defaults, main and master. the first that exists as a branch of repo, or of the remote, wins. both are blank
// when repo has none.
func DefaultBase(repo *git.Repository, remote string) (name, commit string) {
	tracked := plumbing.NewRemoteReferenceName(remote, "").String()
	heads := plumbing.NewBranchReferenceName("").String()

	candidates := make([]string, 0, 4)
	if ref, err := repo.Reference(plumbing.NewRemoteHEADReferenceName(remote), false); err == nil && ref.Type() == plumbing.SymbolicReference {
		candidates = append(candidates, strings.TrimPrefix(strings.TrimPrefix(ref.Target().String(), tracked), heads))
	}

	if cfg, err := repo.Config(); err == nil && cfg.Init.DefaultBranch != "" {
		candidates = append(candidates, cfg.Init.DefaultBranch)
	}

	candidates = append(candidates, plumbing.Main.Short(), plumbing.Master.Short())

	for _, candidate := range candidates {
		for _, ref := range []plumbing.ReferenceName{plumbing.NewBranchReferenceName(candidate), plumbing.NewRemoteReferenceName(remote, candidate)} {
			if resolved, err := repo.Reference(ref, true); err == nil {
				return candidate, resolved.Hash().String()
			}
		}
	}

	return "", ""
}

// RepositoryUsername returns the username owning the repository at path,
// falling back to the current user when it can't be determined.
func RepositoryUsername(path string) string {
	fallback := userx.CurrentUsername(eg.DefaultUsername)

	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		debugx.Println("unable to stat repository, using current user", err)
		return fallback
	}

	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fallback
	}

	u, err := user.LookupId(strconv.FormatUint(uint64(st.Uid), 10))
	if err != nil {
		debugx.Println("unable to lookup repository owner, using current user", err)
		return fallback
	}

	return u.Username
}

func Clone(ctx context.Context, dir, uri, remote, treeish string, opts ...client.Option) (err error) {
	var (
		r *git.Repository
	)

	branchRefName := plumbing.NewBranchReferenceName(treeish)

	if r, err = git.PlainOpen(dir); err == nil {
		rm, err := r.Remote(remote)
		if err != nil {
			return errorsx.Wrapf(err, "unable to find remote: '%s'", remote)
		}

		err = rm.FetchContext(ctx, &git.FetchOptions{ClientOptions: opts})
		uptodate := errors.Is(err, git.NoErrAlreadyUpToDate)
		if err != nil && !uptodate {
			return errorsx.Wrap(err, "unable to fetch")
		}

		fetchBaseOrLog(ctx, r, remote, treeish, opts...)

		if uptodate {
			return nil
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

	if r, err = git.PlainCloneContext(ctx, dir, cloneOpts); err != nil {
		return errorsx.Wrapf(err, "unable to clone: %s - %s", uri, treeish)
	}

	fetchBaseOrLog(ctx, r, remote, treeish, opts...)

	return nil
}

// fetchBaseOrLog is fetchBase, a failure is logged and not returned. the base is optional, see baseenv, so not being
// able to get it, e.g.) the network, mustn't fail the clone.
func fetchBaseOrLog(ctx context.Context, r *git.Repository, remote, treeish string, opts ...client.Option) {
	if err := fetchBase(ctx, r, remote, treeish, opts...); err != nil {
		log.Println(errorsx.Wrap(err, "unable to fetch the base branch"))
	}
}

// fetchBase makes the default branch of the remote available as its remote-tracking branch, and the remote's HEAD,
// which is how DefaultBase finds the base. the single branch clones Clone makes only have the branch cloned, so a
// workload run from any other ref wouldn't have a base. it is only the one branch fetched, the objects the cloned branch
// shares with it are already present and not transferred again. history isn't cut, the base is compared with the head.
func fetchBase(ctx context.Context, r *git.Repository, remote, treeish string, opts ...client.Option) error {
	rm, err := r.Remote(remote)
	if err != nil {
		return errorsx.Wrapf(err, "unable to find remote: '%s'", remote)
	}

	refs, err := rm.ListContext(ctx, &git.ListOptions{ClientOptions: opts})
	if err != nil {
		return errorsx.Wrap(err, "unable to list the remote")
	}

	base := remoteDefaultBranch(refs)
	if base == "" {
		debugx.Println("the remote has no default branch, there is no base to fetch", remote)
		return nil
	}

	tracking := plumbing.NewRemoteReferenceName(remote, base)

	// the branch cloned was just fetched.
	if base != treeish {
		spec := config.RefSpec(fmt.Sprintf("+%s:%s", plumbing.NewBranchReferenceName(base), tracking))
		err = rm.FetchContext(ctx, &git.FetchOptions{RemoteName: remote, RefSpecs: []config.RefSpec{spec}, ClientOptions: opts})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return errorsx.Wrapf(err, "unable to fetch: %s", base)
		}
	}

	return errorsx.Wrap(r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.NewRemoteHEADReferenceName(remote), tracking)), "unable to record the base")
}

// remoteDefaultBranch is the name of the branch the HEAD of the remote refers to, from the references it lists.
// a remote that doesn't say, an older server, has main or master when it has either, blank when it has neither.
func remoteDefaultBranch(refs []*plumbing.Reference) string {
	for _, ref := range refs {
		if ref.Name() == plumbing.HEAD && ref.Type() == plumbing.SymbolicReference {
			return ref.Target().Short()
		}
	}

	for _, candidate := range []string{plumbing.Main.Short(), plumbing.Master.Short()} {
		for _, ref := range refs {
			if ref.Name() == plumbing.NewBranchReferenceName(candidate) {
				return candidate
			}
		}
	}

	return ""
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

	cloneuri := stringsx.First(vcsclone, errorsx.Zero(QuirkCloneURI(repo, remote)))

	if env, err = HeadEnv(repo, uri, cloneuri, branch); err != nil {
		return nil, err
	}

	benv, err := baseenv(repo, remote, uri, cloneuri)
	if err != nil {
		return nil, errorsx.Wrapf(err, "base env: %s", uri)
	}

	return append(env, benv...), nil
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

	if benv, err = baseenv(repo, remote, uri, eg.DefaultWorkingDirectory()); err != nil {
		return nil, errorsx.Wrapf(err, "base env: %s", uri)
	}

	env = append(env, benv...)
	env = append(env, envx.Format(eg.EnvComputeVCS, uri))

	return env, nil
}

// SourceBase is DefaultBase for the repository dir is part of, blank when dir isn't in one.
func SourceBase(dir, remote string) string {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		debugx.Println("unable to open the repository to find its base", dir, err)
		return ""
	}

	name, _ := DefaultBase(repo, remote)
	return name
}

// baseenv describes the base branch of repo, nothing when it has none.
// baremetal runs on github actions failed outright when the checkout had no base (shallow, single branch
// fetches), so it is tolerated and logged. LocalClone carries the base into the working directory, so reaching
// this means the checkout run from never had one.
func baseenv(repo *git.Repository, remote, vcs, uri string) (env []string, err error) {
	name, _ := DefaultBase(repo, remote)
	if name == "" {
		log.Println("unable to resolve the base branch locally or from the remote, omitting the EG_GIT_BASE_* variables")
		return nil, nil
	}

	return BaseEnv(repo, vcs, uri, name)
}

// Description labels a workload for treeish by replacing the eggit style tokens in pattern,
// so callers position them as they like.
// %git.hash%           -> the full hex encoded commit id.
// %git.hash.short%     -> the first 7 characters of the hex encoded commit id.
// %git.commit.author%  -> the committer's name, falling back to their email and then the environment user.
// %git.commit.email%   -> the committer's email.
// %git.ref%            -> the treeish.
// commit values are empty when the reference can't be resolved.
func Description(pattern string, repo *git.Repository, treeish string) string {
	var (
		hash   *plumbing.Hash
		commit *object.Commit
		err    error
		full   string
		author string
		email  string
	)

	if hash, err = repo.ResolveRevision(plumbing.Revision(treeish)); err == nil {
		if commit, err = repo.CommitObject(*hash); err == nil {
			full = commit.Hash.String()
			author = stringsx.First(commit.Committer.Name, commit.Committer.Email)
			email = commit.Committer.Email
		}
	}

	return strings.NewReplacer(
		"%git.hash%", full,
		"%git.hash.short%", full[:min(len(full), 7)],
		"%git.commit.author%", stringsx.First(author, envx.String("unknown", "USER", "USERNAME")),
		"%git.commit.email%", email,
		"%git.ref%", treeish,
	).Replace(pattern)
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

	// checkouts like github actions only have remote-tracking refs for the branches other than the one checked out.
	if hash, err = repo.ResolveRevision(plumbing.Revision(treeish)); errors.Is(err, plumbing.ErrReferenceNotFound) {
		hash, err = repo.ResolveRevision(plumbing.Revision(git.DefaultRemoteName + "/" + treeish))
	}
	if err != nil {
		return nil, errorsx.Wrapf(err, "unable to resolve git revision: %s", treeish)
	}

	if commit, err = repo.CommitObject(*hash); err != nil {
		return nil, errorsx.Wrapf(err, "unable to resolve git reference: %s", treeish)
	}

	return envx.Build().Var(
		eg.EnvGitBaseVCS, vcs,
	).Var(
		eg.EnvGitBaseURI, uri,
	).Var(
		eg.EnvGitBaseRef, treeish,
	).Var(
		eg.EnvGitBaseCommit, commit.Hash.String(),
	).Environ()
}

func vcsuri(uris ...string) string {
	uri := slicesx.FirstOrZero(uris...)
	if u, err := url.Parse(uri); err == nil {
		return fmt.Sprintf("git@%s:%s", u.Host, strings.TrimPrefix(u.Path, "/"))
	}

	uri = strings.TrimPrefix(uri, "ssh://")
	if strings.Contains(uri, "@") && strings.Count(uri, ":") == 0 {
		return strings.Replace(uri, "/", ":", 1)
	}

	return uri
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

// ExchangeCredentials performs a single, immediate exchange of the given
// long-lived access token for short-lived git credentials for vcsuri, kept in
// memory rather than written to disk -- for callers that clone in-process
// right away (e.g. compiling a single workload). Unlike RefreshCredentials a
// blank token is an error. The returned option is nil when the credentials
// don't apply to vcsuri (see LoadCredentials).
func ExchangeCredentials(ctx context.Context, c *http.Client, vcsuri string, token string) (client.Option, error) {
	if stringsx.Blank(token) {
		return nil, errors.New("access token required")
	}

	encoded, err := credentialExchange(ctx, c, token)
	if err != nil {
		return nil, err
	}

	if opt := sshcredentials(ctx, c, vcsuri, token, encoded); opt != nil {
		return opt, nil
	}

	return httpcredentials(vcsuri, encoded), nil
}

func credentialRefresh(ctx context.Context, c *http.Client, dst, token string) error {
	encoded, err := credentialExchange(ctx, c, token)
	if err != nil {
		return err
	}

	if err = os.WriteFile(filepath.Join(dst, "vcsaccess.token"), encoded, 0666); err != nil {
		return errorsx.Wrap(err, "unable to write credentials")
	}

	return nil
}

func credentialExchange(ctx context.Context, c *http.Client, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/r/vcsaccess/", eg.EnvContainerAPIHostDefault()), nil)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to create http request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", token))

	resp, err := httpx.AsError(c.Do(req))
	if err != nil {
		return nil, errorsx.Wrap(err, "http request failed")
	}
	defer resp.Body.Close()
	encoded, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to create http request")
	}

	return encoded, nil
}

func LoadCredentials(ctx context.Context, vcsuri string, dir string) (client.Option, error) {
	encoded, err := os.ReadFile(filepath.Join(dir, "vcsaccess.token"))
	if err != nil {
		return nil, err
	}

	return httpcredentials(vcsuri, encoded), nil
}

// httpcredentials decodes exchanged credentials into http basic auth for
// vcsuri, nil when they're malformed or vcsuri isn't an http uri.
func httpcredentials(vcsuri string, encoded []byte) client.Option {
	var (
		httpauth compute.GitCredentialsHTTP
	)

	if err := json.Unmarshal(encoded, &httpauth); err == nil && stringsx.Present(httpauth.Username) && stringsx.Present(httpauth.Password) {
		if strings.HasPrefix(vcsuri, "http") {
			return client.WithHTTPAuth(&githttp.BasicAuth{Username: httpauth.Username, Password: httpauth.Password})
		}
	}

	return nil
}

// remotesigner is an ssh.Signer that delegates signing to the credential service,
// the private key never leaves the service.
type remotesigner struct {
	ctx    context.Context
	c      *http.Client
	token  string
	pubkey ssh.PublicKey
}

func (t remotesigner) PublicKey() ssh.PublicKey {
	return t.pubkey
}

func (t remotesigner) Sign(r io.Reader, data []byte) (*ssh.Signature, error) {
	return t.SignWithAlgorithm(r, data, "")
}

func (t remotesigner) SignWithAlgorithm(_ io.Reader, data []byte, algorithm string) (*ssh.Signature, error) {
	var (
		sig compute.GitSignatureSSH
	)

	body, err := json.Marshal(&compute.GitSignRequest{Data: data, Algorithm: algorithm})
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to encode signing request")
	}

	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, fmt.Sprintf("%s/r/vcsaccess/sign", eg.EnvContainerAPIHostDefault()), bytes.NewReader(body))
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to create http request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", t.token))
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpx.AsError(t.c.Do(req))
	if err != nil {
		return nil, errorsx.Wrap(err, "signing request failed")
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&sig); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode signature")
	}

	return &ssh.Signature{Format: sig.Format, Blob: sig.Blob}, nil
}

// sshcredentials decodes exchanged credentials into ssh auth for vcsuri,
// nil when they're not ssh credentials or vcsuri isn't an ssh uri.
func sshcredentials(ctx context.Context, c *http.Client, vcsuri string, token string, encoded []byte) client.Option {
	var (
		creds compute.GitCredentialsSSH
	)

	if strings.HasPrefix(vcsuri, "http") {
		return nil
	}

	if err := json.Unmarshal(encoded, &creds); err != nil || len(creds.PublicKey) == 0 {
		return nil
	}

	pubkey, err := ssh.ParsePublicKey(creds.PublicKey)
	if err != nil {
		debugx.Println(errorsx.Wrap(err, "invalid ssh public key"))
		return nil
	}

	return client.WithSSHAuth(&gitssh.PublicKeys{
		User:   creds.Username,
		Signer: remotesigner{ctx: ctx, c: c, token: token, pubkey: pubkey},
	})
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

// gitcmd builds a `git -C repo <args>` invocation, run as the current user --
// repo is frequently bind-mounted into the workload container and owned by
// a different uid than whatever this process runs as, which trips git's
// "dubious ownership" safe.directory check (CVE-2022-24765 mitigation) when
// run directly. see execx.RunAs.
func gitcmd(ctx context.Context, repo string, args ...string) *exec.Cmd {
	return execx.RunAs(ctx, userx.CurrentUsername(eg.DefaultUsername), "git", append([]string{"-C", repo}, args...)...)
}

// Changes returns a patch of the changes made within clonedir, a clone of the repository at repodir,
// e.g.) the changes a workload made, which would otherwise be lost with the clone. new files
// are included. the patch is empty if there are no changes, or if clonedir is not a separate clone,
// the clone is not usable afterwards.
func Changes(ctx context.Context, clonedir string, repodir string) ([]byte, error) {
	if !IsRepository(clonedir) {
		return nil, nil
	}

	// directories that aren't repositories are symlinked instead of cloned.
	if resolved, err := filepath.EvalSymlinks(clonedir); err == nil {
		if original, err := filepath.EvalSymlinks(repodir); err == nil && resolved == original {
			return nil, nil
		}
	}

	// intent to add so new files are reported by diff.
	if out, err := gitcmd(ctx, clonedir, "add", "-N", ".").CombinedOutput(); err != nil {
		return nil, errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err), "unable to detect changes: %s", clonedir)
	}

	patch, err := gitcmd(ctx, clonedir, "diff", "--binary", plumbing.HEAD.String()).Output()
	if err != nil {
		return nil, errorsx.Wrapf(err, "unable to detect changes: %s", clonedir)
	}

	return patch, nil
}

// ApplyPatch applies the patch to the repository at repodir, changes that are already present are
// skipped. if the patch cannot be applied repodir is left untouched and the patch is saved to a file,
// which the returned error names.
func ApplyPatch(ctx context.Context, repodir string, patch []byte) error {
	if len(patch) == 0 {
		return nil
	}

	patchfile, err := os.CreateTemp("", "eg-*.patch")
	if err != nil {
		return err
	}
	defer patchfile.Close()

	if _, err = patchfile.Write(patch); err != nil {
		return errors.Join(err, os.Remove(patchfile.Name()))
	}

	apply := func(options ...string) error {
		args := append([]string{"apply"}, append(options, patchfile.Name())...)
		if out, err := gitcmd(ctx, repodir, args...).CombinedOutput(); err != nil {
			return errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err), "git %s failed", strings.Join(args, " "))
		}

		return nil
	}

	// already present, e.g.) the changes were applied by whatever made them.
	if err := apply("--reverse", "--check"); err == nil {
		return os.Remove(patchfile.Name())
	}

	if err := apply("--check"); err != nil {
		return errorsx.Wrapf(err, "unable to apply changes, they have been saved to %s", patchfile.Name())
	}

	if err := apply("--binary"); err != nil {
		return errorsx.Wrapf(err, "unable to apply changes, they have been saved to %s", patchfile.Name())
	}

	return os.Remove(patchfile.Name())
}
