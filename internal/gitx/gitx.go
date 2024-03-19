package gitx

import (
	"fmt"
	"log"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

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

func Env(dir string, remote string, treeish string) (env []string, err error) {
	commit, err := Commitish(dir, treeish)
	if err != nil {
		return nil, err
	}

	uri, err := Remote(dir, remote)
	if err != nil {
		return nil, err
	}
	env = append(env, fmt.Sprintf("EG_GIT_COMMIT=%s", commit))
	env = append(env, fmt.Sprintf("EG_GIT_REMOTE=%s", uri))

	return env, nil
}
