package duckproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueBytes(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		in := []byte{0x00, 0x01, 0xff, 0x42}

		pv, err := ToProtoValue(in)
		require.NoError(t, err)
		require.Equal(t, in, pv.GetBytesValue())

		got, err := FromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, in, got)
	})

	t.Run("null", func(t *testing.T) {
		pv, err := ToProtoValue(nil)
		require.NoError(t, err)
		require.True(t, pv.GetIsNull())

		got, err := FromProtoValue(pv)
		require.NoError(t, err)
		require.Nil(t, got)
	})
}
