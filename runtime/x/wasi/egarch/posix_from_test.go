package egarch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPOSIXFrom(t *testing.T) {
	t.Run("amd64 maps to x86_64", func(t *testing.T) {
		got := POSIXFrom("amd64")
		require.Equal(t, "x86_64", got)
	})

	t.Run("arm64 maps to aarch64", func(t *testing.T) {
		got := POSIXFrom("arm64")
		require.Equal(t, "aarch64", got)
	})

	t.Run("386 maps to i686", func(t *testing.T) {
		got := POSIXFrom("386")
		require.Equal(t, "i686", got)
	})

	t.Run("arm maps to armhf", func(t *testing.T) {
		got := POSIXFrom("arm")
		require.Equal(t, "armhf", got)
	})

	t.Run("unknown falls back to x86_64", func(t *testing.T) {
		got := POSIXFrom("riscv64")
		require.Equal(t, "x86_64", got)
	})
}
