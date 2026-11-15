package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestRangeFromStrings(t *testing.T) {
	t.Run("should parse both bounds", func(t *testing.T) {
		r := RangeFromStrings("00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002")
		require.Equal(t, Range{
			Begin: MustParse("00000000-0000-0000-0000-000000000001"),
			End:   MustParse("00000000-0000-0000-0000-000000000002"),
		}, r)
	})

	t.Run("should panic when the begin bound is malformed", func(t *testing.T) {
		require.Panics(t, func() { RangeFromStrings("nope", "00000000-0000-0000-0000-000000000002") })
	})

	t.Run("should panic when the end bound is malformed", func(t *testing.T) {
		require.Panics(t, func() { RangeFromStrings("00000000-0000-0000-0000-000000000001", "nope") })
	})
}

func TestRangeEverything(t *testing.T) {
	t.Run("should span from Nil to Max", func(t *testing.T) {
		require.Equal(t, Range{Begin: Nil(), End: Max()}, RangeEverything())
	})
}
