package eggolang

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestWorkspacePath(t *testing.T) {
	t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/ws")

	require.Equal(t, "a/b.go", workspacepath("/ws/a/b.go"))
	require.Equal(t, "/elsewhere/b.go", workspacepath("/elsewhere/b.go"))
	require.Equal(t, "/ws-other/b.go", workspacepath("/ws-other/b.go"))
	require.Equal(t, "example.com/mod/b.go", workspacepath("example.com/mod/b.go"))
}
