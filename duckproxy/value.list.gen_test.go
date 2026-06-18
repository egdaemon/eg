package duckproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueList(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		in := []any{int64(1), "two", true, nil}

		pv, err := toProtoValue(in)
		require.NoError(t, err)
		require.Len(t, pv.GetListValue().GetValues(), len(in))

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, in, got)
	})

	t.Run("empty", func(t *testing.T) {
		pv, err := toProtoValue([]any{})
		require.NoError(t, err)

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, []any{}, got)
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
