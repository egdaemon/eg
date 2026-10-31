package egenv_test

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestPathRelativeWorking(t *testing.T) {
	t.Setenv(_eg.EnvComputeWorkingDirectory, "/wd")

	require.Equal(t, "a/b.go", egenv.PathRelativeWorking("/wd/a/b.go"))
	require.Equal(t, "/elsewhere/b.go", egenv.PathRelativeWorking("/elsewhere/b.go"))
	require.Equal(t, "/wd-other/b.go", egenv.PathRelativeWorking("/wd-other/b.go"))
	require.Equal(t, "example.com/mod/b.go", egenv.PathRelativeWorking("example.com/mod/b.go"))
}
