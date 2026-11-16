package execx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArgs(t *testing.T) {
	t.Run("appends to the arguments", func(t *testing.T) {
		require.Equal(t, args{"reset-failed", "--wait", "a.service", "b.mount"}, Args("reset-failed", "--wait").Append("a.service", "b.mount"))
	})

	t.Run("appending nothing is the arguments", func(t *testing.T) {
		require.Equal(t, args{"reset-failed", "--wait"}, Args("reset-failed", "--wait").Append())
	})

	t.Run("does not change the arguments it was called on", func(t *testing.T) {
		base := make([]string, 2, 8)
		base[0], base[1] = "reset-failed", "--wait"

		first := Args(base...).Append("a")
		second := Args(base...).Append("b")

		require.Equal(t, args{"reset-failed", "--wait", "a"}, first)
		require.Equal(t, args{"reset-failed", "--wait", "b"}, second)
		require.Equal(t, []string{"reset-failed", "--wait"}, base)
	})
}
