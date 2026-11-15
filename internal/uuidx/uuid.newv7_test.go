package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestNewV7(t *testing.T) {
	t.Run("should generate a version 7 uuid", func(t *testing.T) {
		require.Equal(t, V7, NewV7().Version())
	})

	t.Run("should not generate the zero uuid", func(t *testing.T) {
		require.False(t, NewV7().IsZero())
	})

	t.Run("should generate unique values", func(t *testing.T) {
		require.NotEqual(t, NewV7(), NewV7())
	})
}
