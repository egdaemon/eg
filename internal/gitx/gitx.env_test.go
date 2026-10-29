package gitx_test

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/gitx"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/require"
)

// collects the environment into a map, failing if a key is defined more than once.
func environMap(t *testing.T, environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, v := range environ {
		k, val, _ := strings.Cut(v, "=")
		_, dup := m[k]
		require.False(t, dup, "duplicate environment variable: %s", k)
		m[k] = val
	}
	return m
}

const testvcs = "git@github.com:egdaemon/eg.git"

func newEnvRepo(t *testing.T) (*git.Repository, plumbing.Hash) {
	repo, err := git.PlainInit(t.TempDir(), false)
	require.NoError(t, err)

	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/egdaemon/eg.git"}})
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	sig := &object.Signature{Name: "Jane", Email: "jane@example.com", When: time.Now()}
	h, err := wt.Commit("msg", &git.CommitOptions{AllowEmptyCommits: true, Author: sig, Committer: sig})
	require.NoError(t, err)

	return repo, h
}

func TestLocalEnv(t *testing.T) {
	const vcs = testvcs
	newrepo := newEnvRepo

	t.Run("head and base are described with distinct vcs and uri variables", func(t *testing.T) {
		repo, h := newrepo(t)
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h)))

		environ, err := gitx.LocalEnv(repo, "origin", "HEAD")
		require.NoError(t, err)

		env := environMap(t, environ)
		require.Equal(t, vcs, env["EG_GIT_HEAD_VCS"])
		require.Equal(t, "/workload/eg", env["EG_GIT_HEAD_URI"])
		require.Equal(t, "HEAD", env["EG_GIT_HEAD_REF"])
		require.Equal(t, h.String(), env["EG_GIT_HEAD_COMMIT"])
		require.Equal(t, vcs, env["EG_GIT_BASE_VCS"])
		require.Equal(t, "/workload/eg", env["EG_GIT_BASE_URI"])
		require.Equal(t, "main", env["EG_GIT_BASE_REF"])
		require.Equal(t, h.String(), env["EG_GIT_BASE_COMMIT"])
		require.Equal(t, vcs, env["EG_COMPUTE_VCS_URI"])
	})

	t.Run("a missing base branch omits the base variables", func(t *testing.T) {
		repo, h := newrepo(t)
		// ensure there is no main branch regardless of go-git's default branch.
		_ = repo.Storer.RemoveReference(plumbing.NewBranchReferenceName("main"))

		environ, err := gitx.LocalEnv(repo, "origin", "HEAD")
		require.NoError(t, err)

		env := environMap(t, environ)
		require.Equal(t, h.String(), env["EG_GIT_HEAD_COMMIT"])
		for k := range env {
			require.False(t, strings.HasPrefix(k, "EG_GIT_BASE_"), "unexpected base variable: %s", k)
		}
	})

	t.Run("a base branch only available as a remote-tracking ref is used", func(t *testing.T) {
		repo, h := newrepo(t)
		_ = repo.Storer.RemoveReference(plumbing.NewBranchReferenceName("main"))
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", "main"), h)))

		environ, err := gitx.LocalEnv(repo, "origin", "HEAD")
		require.NoError(t, err)

		env := environMap(t, environ)
		require.Equal(t, testvcs, env["EG_GIT_BASE_VCS"])
		require.NotEmpty(t, env["EG_GIT_BASE_URI"])
		require.Equal(t, "main", env["EG_GIT_BASE_REF"])
		require.Equal(t, h.String(), env["EG_GIT_BASE_COMMIT"])
	})

	t.Run("a missing base branch is logged", func(t *testing.T) {
		repo, _ := newrepo(t)
		_ = repo.Storer.RemoveReference(plumbing.NewBranchReferenceName("main"))

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		_, err := gitx.LocalEnv(repo, "origin", "HEAD")
		require.NoError(t, err)
		require.Contains(t, logged.String(), "EG_GIT_BASE_")
	})
}

func TestEnv(t *testing.T) {
	t.Run("describes head and base, using the quirked clone uri", func(t *testing.T) {
		repo, h := newEnvRepo(t)
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h)))

		environ, err := gitx.Env(repo, "origin", "HEAD", "")
		require.NoError(t, err)

		env := environMap(t, environ)
		require.Equal(t, h.String(), env["EG_GIT_HEAD_COMMIT"])
		require.Equal(t, "HEAD", env["EG_GIT_HEAD_REF"])
		require.Equal(t, "Jane", env["EG_GIT_HEAD_COMMIT_AUTHOR"])
		require.Equal(t, "jane@example.com", env["EG_GIT_HEAD_COMMIT_EMAIL"])
		require.Equal(t, "https://github.com/egdaemon/eg.git", env["EG_GIT_HEAD_URI"])
		require.Equal(t, testvcs, env["EG_GIT_BASE_VCS"])
		require.Equal(t, "https://github.com/egdaemon/eg.git", env["EG_GIT_BASE_URI"])
		require.Equal(t, "main", env["EG_GIT_BASE_REF"])
		require.Equal(t, h.String(), env["EG_GIT_BASE_COMMIT"])
	})

	t.Run("a missing base branch omits the base variables", func(t *testing.T) {
		repo, h := newEnvRepo(t)
		// ensure there is no main branch regardless of go-git's default branch.
		_ = repo.Storer.RemoveReference(plumbing.NewBranchReferenceName("main"))

		environ, err := gitx.Env(repo, "origin", "HEAD", "")
		require.NoError(t, err)

		env := environMap(t, environ)
		require.Equal(t, h.String(), env["EG_GIT_HEAD_COMMIT"])
		for k := range env {
			require.False(t, strings.HasPrefix(k, "EG_GIT_BASE_"), "unexpected base variable: %s", k)
		}
	})

	t.Run("a base branch only available as a remote-tracking ref is used", func(t *testing.T) {
		repo, h := newEnvRepo(t)
		_ = repo.Storer.RemoveReference(plumbing.NewBranchReferenceName("main"))
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", "main"), h)))

		environ, err := gitx.Env(repo, "origin", "HEAD", "")
		require.NoError(t, err)

		env := environMap(t, environ)
		require.Equal(t, testvcs, env["EG_GIT_BASE_VCS"])
		require.NotEmpty(t, env["EG_GIT_BASE_URI"])
		require.Equal(t, "main", env["EG_GIT_BASE_REF"])
		require.Equal(t, h.String(), env["EG_GIT_BASE_COMMIT"])
	})

	t.Run("a missing base branch is logged", func(t *testing.T) {
		repo, _ := newEnvRepo(t)
		_ = repo.Storer.RemoveReference(plumbing.NewBranchReferenceName("main"))

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		_, err := gitx.Env(repo, "origin", "HEAD", "")
		require.NoError(t, err)
		require.Contains(t, logged.String(), "EG_GIT_BASE_")
	})

	t.Run("an explicit clone uri takes precedence", func(t *testing.T) {
		repo, _ := newEnvRepo(t)

		environ, err := gitx.Env(repo, "origin", "HEAD", "https://example.com/clone.git")
		require.NoError(t, err)
		require.Equal(t, "https://example.com/clone.git", environMap(t, environ)["EG_GIT_HEAD_URI"])
	})

	t.Run("unknown remote fails", func(t *testing.T) {
		repo, _ := newEnvRepo(t)
		_, err := gitx.Env(repo, "missing", "HEAD", "")
		require.Error(t, err)
	})
}

func TestHeadAndBaseEnvErrors(t *testing.T) {
	repo, h := newEnvRepo(t)

	t.Run("unresolvable revisions fail", func(t *testing.T) {
		_, err := gitx.HeadEnv(repo, testvcs, "/workload/eg", "nope")
		require.Error(t, err)

		_, err = gitx.BaseEnv(repo, testvcs, "/workload/eg", "nope")
		require.Error(t, err)
	})

	t.Run("a missing reference is detectable, LocalEnv relies on it", func(t *testing.T) {
		_, err := gitx.BaseEnv(repo, testvcs, "/workload/eg", "main")
		require.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	})

	t.Run("base uses distinct vcs and uri keys", func(t *testing.T) {
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h)))

		environ, err := gitx.BaseEnv(repo, testvcs, "/workload/eg", "main")
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			"EG_GIT_BASE_VCS":    testvcs,
			"EG_GIT_BASE_URI":    "/workload/eg",
			"EG_GIT_BASE_REF":    "main",
			"EG_GIT_BASE_COMMIT": h.String(),
		}, environMap(t, environ))
	})
}

func TestLocalEnvErrors(t *testing.T) {
	repo, _ := newEnvRepo(t)

	t.Run("unknown remote fails", func(t *testing.T) {
		_, err := gitx.LocalEnv(repo, "missing", "HEAD")
		require.Error(t, err)
	})

	t.Run("unresolvable head fails", func(t *testing.T) {
		_, err := gitx.LocalEnv(repo, "origin", "nope")
		require.Error(t, err)
	})
}

func TestCanonicalURI(t *testing.T) {
	repo, _ := newEnvRepo(t)

	t.Run("normalizes to the git@host:path form", func(t *testing.T) {
		uri, err := gitx.CanonicalURI(repo, "origin")
		require.NoError(t, err)
		require.Equal(t, testvcs, uri)
	})

	t.Run("unknown remote fails", func(t *testing.T) {
		_, err := gitx.CanonicalURI(repo, "missing")
		require.Error(t, err)
	})
}

func TestQuirkCloneURI(t *testing.T) {
	repo, err := git.PlainInit(t.TempDir(), false)
	require.NoError(t, err)

	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "github", URLs: []string{"ssh://git@github.com/egdaemon/eg.git"}})
	require.NoError(t, err)
	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "gitlab", URLs: []string{"https://gitlab.com/org/repo.git"}})
	require.NoError(t, err)

	t.Run("github uses the https clone url", func(t *testing.T) {
		uri, err := gitx.QuirkCloneURI(repo, "github")
		require.NoError(t, err)
		require.Equal(t, "https://github.com/egdaemon/eg.git", uri)
	})

	t.Run("other forges keep the canonical uri", func(t *testing.T) {
		uri, err := gitx.QuirkCloneURI(repo, "gitlab")
		require.NoError(t, err)
		require.Equal(t, "git@gitlab.com:org/repo.git", uri)
	})

	t.Run("unknown remote fails", func(t *testing.T) {
		_, err := gitx.QuirkCloneURI(repo, "missing")
		require.Error(t, err)
	})
}
