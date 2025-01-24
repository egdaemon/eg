package egdebuild_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
	"github.com/stretchr/testify/require"
)

func TestBuildContainer(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	// TODO: enable it to test to completion.
	// right now we're just testing up to the call to build the module.
	require.Error(t, egdebuild.Prepare(egdebuild.Runner(), nil)(ctx, egtest.Op()), ffierrors.ErrNotImplemented)
	s := testx.ReadMD5(os.TempDir(), "Containerfile")
	require.Equal(t, testx.ReadMD5(filepath.Join(".debian", "Containerfile")), s)
}
