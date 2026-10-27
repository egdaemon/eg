package eggit

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) {
	const fullhash = "211a6f2a398230a29830a31a9cbf422be74d407f"

	t.Run("hash is zero", func(t *testing.T) {
		require.True(t, hash{}.IsZero())
		require.True(t, nhash("").IsZero())
		require.False(t, nhash(testcommit1().Hash.String()).IsZero())
	})

	t.Run("string replace uses the environment commit", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, fullhash, func() {
			testx.Tempenvvar(_eg.EnvComputeVCS, "https://example.com/repo.git", func() {
				require.Equal(t, "211a6f2", StringReplace("%git.hash.short%"))
				require.Equal(t, fullhash, StringReplace("%git.hash%"))
				require.Equal(t, "https://example.com/repo.git", StringReplace("%eg.git.canonical.uri%"))
			})
		})
	})

	t.Run("canonical uri", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvComputeVCS, "https://example.com/repo.git", func() {
			require.Equal(t, "https://example.com/repo.git", EnvCanonicalURI())
		})
	})

	t.Run("commit from the environment", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, fullhash, func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitAuthor, "EGd engineering", func() {
				testx.Tempenvvar(_eg.EnvGitHeadCommitEmail, "engineering@egdaemon.com", func() {
					testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "2006-01-02T15:04:05Z", func() {
						c := EnvCommit()
						require.Equal(t, testcommit1().Hash, c.Hash)
						require.Equal(t, testcommit1().Committer, c.Committer)
					})
				})
			})
		})
	})
}
