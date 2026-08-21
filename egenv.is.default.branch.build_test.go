package eg_test

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestIsDefaultBranchBuild(t *testing.T) {
	t.Run("equal head and base commits", func(t *testing.T) {
		require.True(t, eg.IsDefaultBranchBuild(
			eg.EnvGitHeadCommit+"=abc123",
			eg.EnvGitBaseCommit+"=abc123",
		))
	})

	t.Run("differing commits, e.g. a PR build", func(t *testing.T) {
		require.False(t, eg.IsDefaultBranchBuild(
			eg.EnvGitHeadCommit+"=abc123",
			eg.EnvGitBaseCommit+"=def456",
		))
	})

	t.Run("missing environment", func(t *testing.T) {
		require.False(t, eg.IsDefaultBranchBuild())
	})

	t.Run("only head set", func(t *testing.T) {
		require.False(t, eg.IsDefaultBranchBuild(eg.EnvGitHeadCommit+"=abc123"))
	})
}
