package egdebuild_test

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
	"github.com/stretchr/testify/require"
)

//go:embed .debian
var debskel embed.FS

func TestBuildContainer(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	// TODO: enable it to test to completion.
	// right now we're just testing up to the call to build the module.
	require.Error(t, egdebuild.Prepare(egdebuild.Runner(), testx.Must(fs.Sub(debskel, ".debian")))(ctx, egtest.Op()), ffierrors.ErrNotImplemented)
	s := testx.ReadMD5(os.TempDir(), ".debian", "Containerfile")
	require.Equal(t, testx.ReadMD5(filepath.Join(".debian", "Containerfile")), s)
}
