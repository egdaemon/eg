package eg_test

import (
	"testing"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/stretchr/testify/require"
)

func TestUnsafeTranspiledRef(t *testing.T) {
	t.Run("id is the provided name", func(t *testing.T) {
		require.Equal(t, "derp", eg.UnsafeTranspiledRef("derp", nil).ID())
	})
}
