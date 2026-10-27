package execx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestString(t *testing.T) {
	t.Run("captures stdout", func(t *testing.T) {
		out, err := String(t.Context(), "printf", "%s-%s", "a", "b")
		require.NoError(t, err)
		require.Equal(t, "a-b", out)
	})

	t.Run("failing command", func(t *testing.T) {
		out, err := String(t.Context(), "false")
		require.Error(t, err)
		require.Empty(t, out)
	})

	t.Run("missing binary", func(t *testing.T) {
		_, err := String(t.Context(), "eg-does-not-exist")
		require.Error(t, err)
	})
}
