package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestSegmenter(t *testing.T) {
	t.Run("should generate all segments", func(t *testing.T) {
		ranges := make([]Range, 0, 1024)
		for i, s := 0, NewSegmenter(); s.Next(); i++ {
			ranges = append(ranges, s.Range())
		}

		require.Equal(t, 1024, len(ranges), "Expected number of segments to be 1024")

		require.Equal(t, Range{
			Begin: Nil(),
			End:   Advance16(Nil(), 64),
		}, ranges[0], "First segment range mismatch")

		require.Equal(t, Range{
			Begin: FromStringOrNil("ffc00000-0000-0000-0000-000000000000"),
			End:   Max(),
		}, ranges[1023], "Last segment range mismatch")
	})
}
