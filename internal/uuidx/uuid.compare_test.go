package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestCompare(t *testing.T) {
	t.Run("should return 0 for equal uuids", func(t *testing.T) {
		require.Equal(t, 0, MustParse("00000000-0000-0000-0000-000000000001").Compare(MustParse("00000000-0000-0000-0000-000000000001")))
	})

	t.Run("should return -1 when less", func(t *testing.T) {
		require.Equal(t, -1, MustParse("00000000-0000-0000-0000-000000000001").Compare(MustParse("00000000-0000-0000-0000-000000000002")))
	})

	t.Run("should return 1 when greater", func(t *testing.T) {
		require.Equal(t, 1, MustParse("00000000-0000-0000-0000-000000000002").Compare(MustParse("00000000-0000-0000-0000-000000000001")))
	})

	t.Run("should order by the most significant byte first", func(t *testing.T) {
		require.Equal(t, 1, MustParse("01000000-0000-0000-0000-000000000000").Compare(MustParse("00ffffff-ffff-ffff-ffff-ffffffffffff")))
	})

	t.Run("should order Nil before Max", func(t *testing.T) {
		require.Equal(t, -1, Nil().Compare(Max()))
	})
}
