package egccache_test

import (
	"fmt"
	"os/user"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egccache"
	"github.com/stretchr/testify/require"
)

func TestCleanup(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	current, err := user.Current()
	require.NoError(t, err)
	group := envx.String(current.Username, eg.EnvComputeDefaultGroup)

	runtime := egccache.Runtime()
	rec := shell.NewRecorder(&runtime)
	require.NoError(t, egccache.Cleanup(egccache.CleanupOption().DiskLimit(bytesx.GiB).UnsafeRuntime(runtime)...)(ctx, egtest.Op()))
	require.Equal(t, rec.Result(), fmt.Sprintf(":CCACHE_DIR=%s:sudo:-E -H -u %s -g %s bash -o pipefail -c echo ccache -M 1024m", egccache.CacheDirectory(), current.Username, group))
}
