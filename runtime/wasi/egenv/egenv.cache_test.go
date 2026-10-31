package egenv_test

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestPathRelativeCache(t *testing.T) {
	t.Setenv(_eg.EnvComputeCacheDirectory, "/cache")

	require.Equal(t, "a/b.go", egenv.PathRelativeCache("/cache/a/b.go"))
	require.Equal(t, "/elsewhere/b.go", egenv.PathRelativeCache("/elsewhere/b.go"))
	require.Equal(t, "/cache-other/b.go", egenv.PathRelativeCache("/cache-other/b.go"))
	require.Equal(t, "example.com/mod/b.go", egenv.PathRelativeCache("example.com/mod/b.go"))
}
