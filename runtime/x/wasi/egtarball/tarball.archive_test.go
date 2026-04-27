package egtarball_test

import (
	"path/filepath"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
	"github.com/stretchr/testify/assert"
)

func TestArchive(t *testing.T) {
	workspace := t.TempDir()
	testx.Tempenvvar(_eg.EnvComputeWorkspaceDirectory, workspace, func() {
		testx.Tempenvvar(_eg.EnvGitHeadCommit, "aabbccdd11223344aabbccdd11223344aabbccdd", func() {
			testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "2024-03-15T10:00:00Z", func() {
				pattern := "myarchive.%git.commit.year%.%git.commit.month%.%git.commit.day%%git.hash.short%"
				assert.Equal(t, filepath.Join(workspace, "myarchive.2024.3.15aabbccd"), egtarball.Archive(pattern))
			})
		})
	})

	t.Run("no substitutions", func(t *testing.T) {
		workspace := t.TempDir()
		testx.Tempenvvar(_eg.EnvComputeWorkspaceDirectory, workspace, func() {
			assert.Equal(t, filepath.Join(workspace, "literal"), egtarball.Archive("literal"))
		})
	})
}
