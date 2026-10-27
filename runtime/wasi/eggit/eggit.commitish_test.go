package eggit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommitish(t *testing.T) {
	t.Run("host call unavailable outside of wasi", func(t *testing.T) {
		require.Equal(t, "", Commitish(t.Context(), "HEAD"))
	})
}
