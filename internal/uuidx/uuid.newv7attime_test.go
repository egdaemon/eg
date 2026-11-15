package uuidx_test

import (
	"encoding/binary"
	"testing"
	"time"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestNewV7AtTime(t *testing.T) {
	t.Run("should embed the timestamp in the top 48 bits as milliseconds", func(t *testing.T) {
		ts := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
		u := NewV7AtTime(ts)

		var ms [8]byte
		copy(ms[2:], u[:6])
		require.Equal(t, uint64(ts.UnixMilli()), binary.BigEndian.Uint64(ms[:]))
	})

	t.Run("should remain a version 7 uuid", func(t *testing.T) {
		require.Equal(t, V7, NewV7AtTime(time.Unix(1_600_000_000, 0)).Version())
	})

	t.Run("should order by timestamp", func(t *testing.T) {
		earlier := NewV7AtTime(time.Unix(1_600_000_000, 0))
		later := NewV7AtTime(time.Unix(1_700_000_000, 0))
		require.Equal(t, -1, earlier.Compare(later))
	})
}
