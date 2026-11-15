package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	t.Run("should report 4 for a v4 uuid", func(t *testing.T) {
		require.Equal(t, byte(4), NewV4().Version())
	})

	t.Run("should report 7 for a v7 uuid", func(t *testing.T) {
		require.Equal(t, V7, NewV7().Version())
	})

	t.Run("should read the version from the parsed value", func(t *testing.T) {
		require.Equal(t, byte(5), MustParse("00000000-0000-5000-8000-000000000000").Version())
	})

	t.Run("should report 0 for Nil", func(t *testing.T) {
		require.Equal(t, byte(0), Nil().Version())
	})
}
