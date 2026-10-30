package egenv_test

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceRelative(t *testing.T) {
	t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/ws")

	require.Equal(t, "a/b.go", egenv.PathRelativeWorkspace("/ws/a/b.go"))
	require.Equal(t, "/elsewhere/b.go", egenv.PathRelativeWorkspace("/elsewhere/b.go"))
	require.Equal(t, "/ws-other/b.go", egenv.PathRelativeWorkspace("/ws-other/b.go"))
	require.Equal(t, "example.com/mod/b.go", egenv.PathRelativeWorkspace("example.com/mod/b.go"))
}

func TestPathRelative(t *testing.T) {
	require.Equal(t, "a/b.go", egenv.PathRelative("/dir", "/dir/a/b.go"))
	require.Equal(t, "b.go", egenv.PathRelative("/dir/a", "/dir/a/b.go"))
	require.Equal(t, "/dir-other/b.go", egenv.PathRelative("/dir", "/dir-other/b.go"))
	require.Equal(t, "/elsewhere/b.go", egenv.PathRelative("/dir", "/elsewhere/b.go"))
	require.Equal(t, "a/b.go", egenv.PathRelative("/dir", "a/b.go"))
}
