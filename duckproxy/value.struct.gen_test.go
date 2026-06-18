package duckproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueStruct(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		in := map[string]any{"a": int64(1), "b": "two", "c": true}

		pv, err := toProtoValue(in)
		require.NoError(t, err)
		require.Len(t, pv.GetStructValue().GetFields(), len(in))

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, in, got)
	})

	t.Run("malformed_empty_oneof", func(t *testing.T) {
		_, err := fromProtoValue(&Value{})
		require.Error(t, err)
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
