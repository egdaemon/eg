package duckproxyv2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueBool(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		pv, err := toProtoValue(true)
		require.NoError(t, err)
		require.True(t, pv.GetBoolValue())

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, true, got)
	})

	t.Run("false", func(t *testing.T) {
		pv, err := toProtoValue(false)
		require.NoError(t, err)

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, false, got)
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
