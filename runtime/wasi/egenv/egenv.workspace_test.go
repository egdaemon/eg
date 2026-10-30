package egenv_test

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceRelative(t *testing.T) {
	t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/ws")

	require.Equal(t, "a/b.go", egenv.PathWorkspaceRelative("/ws/a/b.go"))
	require.Equal(t, "/elsewhere/b.go", egenv.PathWorkspaceRelative("/elsewhere/b.go"))
	require.Equal(t, "/ws-other/b.go", egenv.PathWorkspaceRelative("/ws-other/b.go"))
	require.Equal(t, "example.com/mod/b.go", egenv.PathWorkspaceRelative("example.com/mod/b.go"))
}
