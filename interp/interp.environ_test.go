package interp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleEnviron(t *testing.T) {
	t.Run("reads the environment from the runtime directory of the workspace", func(t *testing.T) {
		ws := workspaces.Context{RuntimeDir: t.TempDir()}
		require.NoError(t, os.WriteFile(filepath.Join(ws.RuntimeDir, eg.EnvironFile), []byte("A=1\n"+eg.EnvComputeRuntimeDirectory+"="+eg.DefaultRuntimeDirectory()+"\n"), 0600))

		environ := moduleenviron(ws)
		assert.Contains(t, environ, "A=1")
		assert.Contains(t, environ, eg.EnvComputeRuntimeDirectory+"="+eg.DefaultRuntimeDirectory())
	})

	t.Run("a workspace without an environment file provides nothing", func(t *testing.T) {
		assert.Empty(t, moduleenviron(workspaces.Context{RuntimeDir: t.TempDir()}))
	})
}
