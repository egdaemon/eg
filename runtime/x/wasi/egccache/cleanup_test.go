package egccache_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egccache"
	"github.com/stretchr/testify/require"
)

func TestCleanup(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	runtime := egccache.Runtime()
	rec := shell.NewRecorder(&runtime)
	require.NoError(t, egccache.Cleanup(egccache.CleanupOption().DiskLimit(bytesx.GiB).UnsafeRuntime(runtime)...)(ctx, egtest.Op()))
	require.Equal(t, rec.Result(), fmt.Sprintf(":CCACHE_DIR=%s:sudo:-H -u egd -g egd env CCACHE_DIR=%s bash -c echo ccache -M 1024m", egccache.CacheDirectory(), egccache.CacheDirectory()))
}
