package cmdopts_test

import (
	"testing"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/runners"
	"github.com/stretchr/testify/require"
)

func TestRuntimeResourcesLimits(t *testing.T) {
	host := runners.RuntimeResources{Cores: 8, Memory: 8 * bytesx.GiB, Vram: 4 * bytesx.GiB}

	t.Run("configured resources within the host are used", func(t *testing.T) {
		require.Equal(t, runners.RuntimeResources{Cores: 2, Memory: 4 * bytesx.GiB, Vram: bytesx.GiB}, cmdopts.RuntimeResources{Cores: 2, Memory: 4 * bytesx.GiB, Vram: bytesx.GiB}.Limits(host))
	})

	t.Run("configured resources are capped at the host", func(t *testing.T) {
		require.Equal(t, host, cmdopts.RuntimeResources{Cores: 100, Memory: bytesx.TiB, Vram: bytesx.TiB}.Limits(host))
	})

	t.Run("zero cores and memory are the host, zero vram is none", func(t *testing.T) {
		require.Equal(t, runners.RuntimeResources{Cores: 8, Memory: 8 * bytesx.GiB, Vram: 0}, cmdopts.RuntimeResources{}.Limits(host))
	})
}
