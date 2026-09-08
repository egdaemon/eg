package podmanx_test

import (
	"runtime"
	"testing"

	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/stretchr/testify/require"
)

func TestAutoPlatform(t *testing.T) {
	t.Run("defaults_to_linux_and_host_arch", func(t *testing.T) {
		require.Equal(t, "linux/"+runtime.GOARCH, podmanx.AutoPlatform(""))
	})

	t.Run("uses_provided_arch", func(t *testing.T) {
		require.Equal(t, "linux/amd64", podmanx.AutoPlatform("amd64"))
	})
}
