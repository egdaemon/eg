package duckproxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValueTimestamp(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		in := time.Date(2024, 3, 15, 9, 30, 0, 0, time.UTC)

		pv, err := toProtoValue(in)
		require.NoError(t, err)
		require.NotNil(t, pv.GetTimestampValue())

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.True(t, got.(time.Time).Equal(in))
	})

	t.Run("null", func(t *testing.T) {
		pv, err := toProtoValue(nil)
		require.NoError(t, err)
		require.True(t, pv.GetIsNull())

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Nil(t, got)
	})
}
