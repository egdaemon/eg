package eggithub

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestPatternVersion(t *testing.T) {
	t.Run("replaces date and unix pattern in version string", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, "aabbccdd11223344aabbccdd11223344aabbccdd", func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "2024-03-15T10:00:00Z", func() {
				v := PatternVersion()
				require.Equal(t, "r2024.3.151710496800", v)
			})
		})
	})

	t.Run("returns zero date when timestamp is missing", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, "aabbccdd11223344aabbccdd11223344aabbccdd", func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "", func() {
				v := PatternVersion()
				require.Equal(t, "r1.1.1-62135596800", v)
			})
		})
	})

	t.Run("handles different dates", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "2025-12-25T00:00:00Z", func() {
				v := PatternVersion()
				require.Equal(t, "r2025.12.251766620800", v)
			})
		})
	})

	t.Run("zero timestamp produces zero values", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, "aabbccdd11223344aabbccdd11223344aabbccdd", func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "0000-00-00T00:00:00Z", func() {
				v := PatternVersion()
				require.Equal(t, "r1.1.1-62135596800", v)
			})
		})
	})
}
