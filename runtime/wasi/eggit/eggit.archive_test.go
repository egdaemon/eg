package eggit

import (
	"os"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestArchive(t *testing.T) {
	dir := t.TempDir()
	dest := t.TempDir()
	gitInit(t, dir)
	commit := gitRevParse(t, dir, "HEAD")
	destfile := dest + "/" + commit + ".tar.gz"

	testx.Tempenvvar(_eg.EnvGitHeadCommit, commit, func() {
		err := archive(shell.NewLocal(), dir, destfile)(t.Context(), nil)
		require.NoError(t, err)
		info, err := os.Stat(destfile)
		require.NoError(t, err)
		require.Greater(t, info.Size(), int64(0))
	})
}
