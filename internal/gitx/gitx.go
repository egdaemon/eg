package gitx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/golang-jwt/jwt/v4"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/timex"
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

// return the clone uri handling quirks of specific forges.
// aka: github requires the use of the http clone url for its authentication token.
func QuirkCloneURI(r *git.Repository, name string) (_ string, err error) {
	uri, err := CanonicalURI(r, name)

	replaced := strings.Replace(uri, "git@github.com:", "https://github.com/", -1)

	return replaced, err
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

	return vcsuri(uri), nil
}

func Env(repo *git.Repository, remote string, branch string, vcsclone string) (env []string, err error) {
	uri, err := CanonicalURI(repo, remote)
	if err != nil {
		return nil, err
	}

	return HeadEnv(repo, uri, stringsx.First(vcsclone, uri), branch)
}

// ideally we shouldn't need this but unfortunately go-git doesn't apply instead of rules properly.
// and from the issue tracker that leads to the question of if it works with the git credential helper.
func LocalEnv(repo *git.Repository, remote string, branch string) (env []string, err error) {
	uri, err := CanonicalURI(repo, remote)
	if err != nil {
		return nil, err
	}

	return HeadEnv(repo, uri, "/opt/eg", branch)
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
	return strings.Replace(strings.TrimPrefix(sshvcsuri(s), "ssh://"), "/", ":", 1)
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
		log.Println("access token blank skipping")
		return nil
	}

	log.Println("periodic git credentials refresh enabled")
	if err := credentialRefresh(ctx, c, dst, token); err != nil {
		return errorsx.Wrap(err, "failed to initially fetch access token")
	}

	go timex.Every(10*time.Minute, func() {
		errorsx.Log(errorsx.Wrap(credentialRefresh(ctx, c, dst, token), "unable to refresh credentials"))
	})

	return nil
}

func credentialRefresh(ctx context.Context, c *http.Client, dst, token string) error {
	// const hostname = "https://host.containers.internal:8081"
	// req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/r/vcsaccess/", hostname), nil)
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

	if err = ioutil.WriteFile(filepath.Join(dst, "vcsaccess.token"), encoded, 0666); err != nil {
		return errorsx.Wrap(err, "unable to write credentials")
	}

	return nil
}

func LoadCredentials(ctx context.Context, vcsuri string, dir string) (transport.AuthMethod, error) {
	var (
		httpauth compute.GitCredentialsHTTP
	)
	encoded, err := ioutil.ReadFile(filepath.Join(dir, "vcsaccess.token"))
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(encoded, &httpauth); err == nil && stringsx.Present(httpauth.Username) && stringsx.Present(httpauth.Password) {
		if strings.HasPrefix(vcsuri, "http") {
			return &githttp.BasicAuth{Username: httpauth.Username, Password: httpauth.Password}, nil
		}
	}

	return nil, nil
}
