package duckproxyv2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueString(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		pv, err := toProtoValue("hello world")
		require.NoError(t, err)
		require.Equal(t, "hello world", pv.GetStringValue())

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, "hello world", got)
	})

	t.Run("empty_string", func(t *testing.T) {
		pv, err := toProtoValue("")
		require.NoError(t, err)

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, "", got)
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
