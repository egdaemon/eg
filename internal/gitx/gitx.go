package gitx

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

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

func Remote(dir, name string) (_ string, err error) {
	var (
		r *git.Repository
	)

	if r, err = git.PlainOpen(dir); err != nil {
		return "", errorsx.Wrapf(err, "unable to detect git repository: %s", dir)
	}

	remote, err := r.Remote(name)
	if err != nil {
		return "", errorsx.Wrapf(err, "unable to detect remote: %s - %s", name, dir)
	}

	return slicesx.FirstOrZero(remote.Config().URLs...), nil
}

func Env(dir string, remote string, branch string) (env []string, err error) {
	commit, err := Commitish(dir, branch)
	if err != nil {
		return nil, err
	}

	uri, err := Remote(dir, remote)
	if err != nil {
		return nil, err
	}

	return HeadEnv(vcsuri(uri), uri, branch, commit)
}

// ideally we shouldn't need this but unfortunately go-git doesn't apply instead of rules properly.
// and from the issue tracker that leads to the question of if it works with the git credential helper.
func LocalEnv(dir string, remote string, branch string) (env []string, err error) {
	commit, err := Commitish(dir, branch)
	if err != nil {
		return nil, err
	}

	uri, err := Remote(dir, remote)
	if err != nil {
		return nil, err
	}

	uri = sshvcsuri(uri)

	return HeadEnv(strings.TrimPrefix(uri, "ssh://"), dir, branch, commit)
}

func HeadEnv(vcs, uri string, ref, commit string) (env []string, err error) {
	env = append(env, fmt.Sprintf("EG_GIT_VCS=%s", vcs))
	env = append(env, fmt.Sprintf("EG_GIT_URI=%s", uri))
	env = append(env, fmt.Sprintf("EG_GIT_REF=%s", ref))
	env = append(env, fmt.Sprintf("EG_GIT_COMMIT=%s", commit))
	return env, nil
}

func sshvcsuri(s string) string {
	vcs := errorsx.Zero(url.Parse(s))
	vcs.Scheme = "ssh"
	vcs.User = url.User("git")
	return vcs.String()
}

func vcsuri(s string) string {
	return strings.TrimPrefix(sshvcsuri(s), "ssh://")
}
