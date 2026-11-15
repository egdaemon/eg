package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestWithSuffix(t *testing.T) {
	t.Run("should zero pad the suffix to twelve digits", func(t *testing.T) {
		require.Equal(t, "00000000-0000-0000-0000-000000000042", WithSuffix(42))
	})

	t.Run("should produce the Nil uuid for zero", func(t *testing.T) {
		require.Equal(t, Nil().String(), WithSuffix(0))
	})

	t.Run("should parse as a valid uuid", func(t *testing.T) {
		_, err := Parse(WithSuffix(123456789012))
		require.NoError(t, err)
	})
}
