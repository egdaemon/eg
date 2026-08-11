package runners_test

import (
	"runtime"
	"testing"

	"github.com/egdaemon/eg/runners"
	"github.com/stretchr/testify/require"
)

func TestNewRuntimeResources(t *testing.T) {
	got := runners.NewRuntimeResources()
	require.Equal(t, uint64(runtime.NumCPU()), got.Cores)
	require.Greater(t, got.Memory, uint64(0))
}

func TestRuntimeResources(t *testing.T) {
	t.Run("NewRuntimeResourcesFromDequeued copies cores/memory/vram", func(t *testing.T) {
		d := &runners.Enqueued{Cores: 2, Memory: 1024, Vram: 4096}
		got := runners.NewRuntimeResourcesFromDequeued(d)
		require.Equal(t, runners.RuntimeResources{Cores: 2, Memory: 1024, Vram: 4096}, got)
	})

	t.Run("Reserve and Release account for cores/memory/vram", func(t *testing.T) {
		base := runners.RuntimeResources{}
		reserved := base.Reserve(runners.RuntimeResources{Cores: 1, Memory: 2, Vram: 3})
		require.Equal(t, runners.RuntimeResources{Cores: 1, Memory: 2, Vram: 3}, reserved)

		released := reserved.Release(runners.RuntimeResources{Cores: 1, Memory: 2, Vram: 3})
		require.Equal(t, runners.RuntimeResources{}, released)
	})

	t.Run("Reserve and Release do not affect unrelated fields", func(t *testing.T) {
		base := runners.RuntimeResources{Cores: 4, Memory: 4096, Vram: 8192}

		reserved := base.Reserve(runners.RuntimeResources{Cores: 1})
		require.Equal(t, runners.RuntimeResources{Cores: 5, Memory: 4096, Vram: 8192}, reserved)

		released := base.Release(runners.RuntimeResources{Memory: 1024})
		require.Equal(t, runners.RuntimeResources{Cores: 4, Memory: 3072, Vram: 8192}, released)
	})

	t.Run("ResourceManager Reserve/Release/Snapshot round trip cores/memory/vram", func(t *testing.T) {
		rm := runners.NewResourceManager(runners.RuntimeResources{Cores: 4, Memory: 4096, Vram: 8192})

		rm.Reserve(runners.RuntimeResources{Cores: 1, Memory: 512, Vram: 2048})
		require.Equal(t, runners.RuntimeResources{Cores: 1, Memory: 512, Vram: 2048}, rm.Snapshot())

		rm.Release(runners.RuntimeResources{Cores: 1, Memory: 512, Vram: 2048})
		require.Equal(t, runners.RuntimeResources{}, rm.Snapshot())
	})
}
