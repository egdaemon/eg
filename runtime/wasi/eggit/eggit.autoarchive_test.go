package eggit

import (
	"os"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestAutoArchive(t *testing.T) {
	t.Run("archives the working directory into the workspace directory", func(t *testing.T) {
		workdir := t.TempDir()
		workspacedir := t.TempDir()
		gitInit(t, workdir) // simulate AutoClone
		commit := gitRevParse(t, workdir, "HEAD")

		testx.Tempenvvar(_eg.EnvComputeWorkingDirectory, workdir, func() {
			testx.Tempenvvar(_eg.EnvComputeWorkspaceDirectory, workspacedir, func() {
				testx.Tempenvvar(_eg.EnvGitHeadCommit, commit, func() {
					require.NoError(t, autoarchive(shell.NewLocal())(t.Context(), nil))

					info, err := os.Stat(egenv.WorkspaceDirectory(commit + ".tar.gz"))
					require.NoError(t, err)
					require.Greater(t, info.Size(), int64(0))
				})
			})
		})
	})
}
