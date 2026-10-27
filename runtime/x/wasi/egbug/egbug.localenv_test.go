package egbug

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/require"
)

func gitkeys(environ []string) []string {
	result := []string{}
	for _, v := range environ {
		if k, _, _ := strings.Cut(v, "="); strings.HasPrefix(k, "EG_GIT_") || k == "EG_COMPUTE_VCS_URI" {
			result = append(result, v)
		}
	}
	sort.Strings(result)
	return result
}

// the git variables produced for local/baremetal runs must normalize to default.env.
// guards against gitx and the defaults drifting apart (missing or duplicated keys).
func TestNormalizeEnvMatchesLocalEnv(t *testing.T) {
	repo, err := git.PlainInit(t.TempDir(), false)
	require.NoError(t, err)

	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/egdaemon/eg.git"}})
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	sig := &object.Signature{Name: "Jane", Email: "jane@example.com", When: time.Now()}
	h, err := wt.Commit("msg", &git.CommitOptions{AllowEmptyCommits: true, Author: sig, Committer: sig})
	require.NoError(t, err)
	require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h)))

	environ, err := gitx.LocalEnv(repo, "origin", "HEAD")
	require.NoError(t, err)

	actual, err := normalizeEnv(envx.Build().FromEnviron(environ...)).Environ()
	require.NoError(t, err)

	// EG_UNSAFE_GIT_CLONE_ENABLED is set by the launcher, not gitx.
	expected := gitkeys(defaultEnviron(t))
	filtered := expected[:0]
	for _, v := range expected {
		if !strings.HasPrefix(v, "EG_UNSAFE_") {
			filtered = append(filtered, v)
		}
	}

	require.Equal(t, filtered, gitkeys(actual))
}
