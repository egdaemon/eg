package gitx_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/gitx"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/stretchr/testify/require"
)

func TestDefaultBase(t *testing.T) {
	t.Run("the branch the remote head points at comes first", func(t *testing.T) {
		repo, h := newEnvRepo(t)
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h)))
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", "develop"), h)))
		require.NoError(t, repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.NewRemoteHEADReferenceName("origin"), plumbing.NewRemoteReferenceName("origin", "develop"))))

		name, commit := gitx.DefaultBase(repo, "origin")
		require.Equal(t, "develop", name)
		require.Equal(t, h.String(), commit)
	})

	t.Run("a remote head that does not resolve is skipped", func(t *testing.T) {
		repo, h := newEnvRepo(t)
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h)))
		require.NoError(t, repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.NewRemoteHEADReferenceName("origin"), plumbing.NewRemoteReferenceName("origin", "gone"))))

		name, commit := gitx.DefaultBase(repo, "origin")
		require.Equal(t, "main", name)
		require.Equal(t, h.String(), commit)
	})

	t.Run("the configured default branch comes before the go-git defaults", func(t *testing.T) {
		repo, h := newEnvRepo(t)
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h)))
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("trunk"), h)))
		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.Init.DefaultBranch = "trunk"
		require.NoError(t, repo.SetConfig(cfg))

		name, commit := gitx.DefaultBase(repo, "origin")
		require.Equal(t, "trunk", name)
		require.Equal(t, h.String(), commit)
	})

	t.Run("main comes before master", func(t *testing.T) {
		repo, h := newEnvRepo(t)
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.Master, h)))
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.Main, h)))

		name, commit := gitx.DefaultBase(repo, "origin")
		require.Equal(t, plumbing.Main.Short(), name)
		require.Equal(t, h.String(), commit)
	})

	t.Run("master is the last resort", func(t *testing.T) {
		repo, h := newEnvRepo(t)
		require.NoError(t, repo.Storer.RemoveReference(plumbing.Main))
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.Master, h)))

		name, commit := gitx.DefaultBase(repo, "origin")
		require.Equal(t, plumbing.Master.Short(), name)
		require.Equal(t, h.String(), commit)
	})

	t.Run("a base that only exists on the remote resolves", func(t *testing.T) {
		repo, h := newEnvRepo(t)
		require.NoError(t, repo.Storer.RemoveReference(plumbing.Master))
		require.NoError(t, repo.Storer.RemoveReference(plumbing.Main))
		require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", "main"), h)))

		name, commit := gitx.DefaultBase(repo, "origin")
		require.Equal(t, "main", name)
		require.Equal(t, h.String(), commit)
	})

	t.Run("nothing resolving yields nothing", func(t *testing.T) {
		repo, _ := newEnvRepo(t)
		require.NoError(t, repo.Storer.RemoveReference(plumbing.Master))
		require.NoError(t, repo.Storer.RemoveReference(plumbing.Main))

		name, commit := gitx.DefaultBase(repo, "origin")
		require.Empty(t, name)
		require.Empty(t, commit)
	})
}
