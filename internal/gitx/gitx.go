package gitx

import (
	"context"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
)

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

func Clone(ctx context.Context, auth transport.AuthMethod, dir, uri, remote, treeish string) (err error) {
	var (
		r *git.Repository
	)

	branchRefName := plumbing.NewBranchReferenceName(treeish)

	if r, err = git.PlainOpen(dir); err == nil {
		remote, err := r.Remote(remote)
		if err != nil {
			return errorsx.Wrapf(err, "unable to find remote: '%s'", remote)
		}

		if err = remote.FetchContext(ctx, &git.FetchOptions{}); errors.Is(err, git.NoErrAlreadyUpToDate) {
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
		log.Println(errorsx.Wrap(err, "repository is missing attempting clone"))
	}

	_, err = git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:               uri,
		ReferenceName:     branchRefName,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Auth:              auth,
		SingleBranch:      true,
	})

	return errorsx.Wrapf(err, "unable to clone: %s - %s", uri, treeish)
}

// return the canonical URI for a repository according to eg. which is git@host:repository.git
func CanonicalURI(r *git.Repository, name string) (_ string, err error) {
	remote, err := r.Remote(name)
	if err != nil {
		return "", errorsx.Wrapf(err, "unable to detect remote: %s", name)
	}

	uri := slicesx.FirstOrZero(remote.Config().URLs...)

	if strings.ContainsRune(uri, '@') {
		return uri, nil
	}

	return sshvcsuri(uri), nil
}

func Env(repo *git.Repository, remote string, branch string) (env []string, err error) {
	uri, err := CanonicalURI(repo, remote)
	if err != nil {
		return nil, err
	}

	return HeadEnv(repo, vcsuri(uri), uri, branch)
}

// ideally we shouldn't need this but unfortunately go-git doesn't apply instead of rules properly.
// and from the issue tracker that leads to the question of if it works with the git credential helper.
func LocalEnv(repo *git.Repository, remote string, branch string) (env []string, err error) {
	uri, err := CanonicalURI(repo, remote)
	if err != nil {
		return nil, err
	}

	return HeadEnv(repo, vcsuri(uri), "/opt/eg", branch)
}

func HeadEnv(repo *git.Repository, vcs, uri string, treeish string) (env []string, err error) {
	var (
		hash   *plumbing.Hash
		commit *object.Commit
	)

	if hash, err = repo.ResolveRevision(plumbing.Revision(treeish)); err != nil {
		return nil, errorsx.Wrapf(err, "unable to resolve git reference: %s", treeish)
	} else if commit, err = repo.CommitObject(*hash); err != nil {
		return nil, errorsx.Wrapf(err, "unable to resolve git reference: %s", treeish)
	}

	return envx.Build().Var(
		"EG_GIT_HEAD_VCS", vcs,
	).Var(
		"EG_GIT_HEAD_URI", uri,
	).Var(
		"EG_GIT_HEAD_REF", treeish,
	).Var(
		"EG_GIT_HEAD_COMMIT", commit.Hash.String(),
	).Var(
		"EG_GIT_HEAD_COMMIT_AUTHOR", commit.Committer.Name,
	).Var(
		"EG_GIT_HEAD_COMMIT_EMAIL", commit.Committer.Email,
	).Var(
		"EG_GIT_HEAD_COMMIT_TIMESTAMP", commit.Committer.When.Format(time.RFC3339),
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

func vcsuri(s string) string {
	return strings.TrimPrefix(sshvcsuri(s), "ssh://")
}
