package egarch

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHost(t *testing.T) {
	t.Run("returns a known architecture string", func(t *testing.T) {
		arch := Host()
		require.NotEmpty(t, arch, "Host() should return a non-empty string")
		require.Equal(t, runtime.GOARCH, arch)
	})
}
