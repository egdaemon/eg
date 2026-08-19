package egarch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDartFrom(t *testing.T) {
	t.Run("386 maps to ia32", func(t *testing.T) {
		require.Equal(t, "ia32", DartFrom("386"))
	})

	t.Run("amd64 maps to x64", func(t *testing.T) {
		require.Equal(t, "x64", DartFrom("amd64"))
	})

	t.Run("arm64 passes through as arm64", func(t *testing.T) {
		require.Equal(t, "arm64", DartFrom("arm64"))
	})

	t.Run("arm passes through as arm", func(t *testing.T) {
		require.Equal(t, "arm", DartFrom("arm"))
	})
}
