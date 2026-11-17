package workspaces_test

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

// the unprivileged user commands run as is in the root group, it must be able to reach the sockets in the runtime directory.
func TestNewRootAndRuntimeDirectoriesAreTraversableByGroup(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")

	ws, err := workspaces.New(t.Context(), sha256.New(), root, root, "test-module")
	require.NoError(t, err)

	for _, dir := range []string{ws.Root, ws.RuntimeDir} {
		st, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0710), st.Mode().Perm()&0710, dir)
		require.Zero(t, st.Mode().Perm()&0007, "others have no access to %s", dir)
	}
}
