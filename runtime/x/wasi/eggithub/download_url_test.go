package eggithub

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestDownloadURL(t *testing.T) {
	t.Run("replaces git SSH URI with HTTPS download URL", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, "aabbccdd11223344aabbccdd11223344aabbccdd", func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "2024-03-15T10:00:00Z", func() {
				testx.Tempenvvar(_eg.EnvComputeVCS, "git@github.com:egdaemon/eg.git", func() {
					u := DownloadURL("eg-{{.Version}}.tar.gz")
					require.Equal(t, "https://github.com/egdaemon/eg/releases/download/r2024.3.151710496800/eg-{{.Version}}.tar.gz", u)
				})
			})
		})
	})

	t.Run("replaces git HTTPS URI with download URL preserving triple slash", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, "aabbccdd11223344aabbccdd11223344aabbccdd", func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "2024-03-15T10:00:00Z", func() {
				testx.Tempenvvar(_eg.EnvComputeVCS, "https://github.com/egdaemon/eg.git", func() {
					u := DownloadURL("eg-{{.Version}}.tar.gz")
					require.Equal(t, "https///github.com/egdaemon/eg/releases/download/r2024.3.151710496800/eg-{{.Version}}.tar.gz", u)
				})
			})
		})
	})

	t.Run("handles different organization and repo name", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "2025-12-25T00:00:00Z", func() {
				testx.Tempenvvar(_eg.EnvComputeVCS, "git@github.com:myorg/myrepo.git", func() {
					u := DownloadURL("myrepo-{{.Version}}.tar.gz")
					require.Equal(t, "https://github.com/myorg/myrepo/releases/download/r2025.12.251766620800/myrepo-{{.Version}}.tar.gz", u)
				})
			})
		})
	})

	t.Run("uses zero time values when timestamp is missing", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, "aabbccdd11223344aabbccdd11223344aabbccdd", func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "", func() {
				testx.Tempenvvar(_eg.EnvComputeVCS, "git@github.com:egdaemon/eg.git", func() {
					u := DownloadURL("eg-{{.Version}}.tar.gz")
					require.Equal(t, "https://github.com/egdaemon/eg/releases/download/r1.1.1-62135596800/eg-{{.Version}}.tar.gz", u)
				})
			})
		})
	})
}
