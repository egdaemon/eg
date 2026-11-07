package unsafecoreutils_test

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/unsafecoreutils"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestShims(t *testing.T) {
	t.Run("without the reactor the utility is not found", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeRuntimeDirectory, t.TempDir())
		require.Equal(t, codes.NotFound, status.Code(unsafecoreutils.Echo(t.Context(), "hello")))
	})
}
