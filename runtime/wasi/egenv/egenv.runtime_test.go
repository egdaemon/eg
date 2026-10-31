package egenv_test

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestPathRelativeRuntime(t *testing.T) {
	t.Setenv(_eg.EnvComputeRuntimeDirectory, "/rt")

	require.Equal(t, "a/b.go", egenv.PathRelativeRuntime("/rt/a/b.go"))
	require.Equal(t, "/elsewhere/b.go", egenv.PathRelativeRuntime("/elsewhere/b.go"))
	require.Equal(t, "/rt-other/b.go", egenv.PathRelativeRuntime("/rt-other/b.go"))
	require.Equal(t, "example.com/mod/b.go", egenv.PathRelativeRuntime("example.com/mod/b.go"))
}
