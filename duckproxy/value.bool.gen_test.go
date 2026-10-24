package duckproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueBool(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		pv, err := ToProtoValue(true)
		require.NoError(t, err)
		require.True(t, pv.GetBoolValue())

		got, err := FromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, true, got)
	})

	t.Run("false", func(t *testing.T) {
		pv, err := ToProtoValue(false)
		require.NoError(t, err)

		got, err := FromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, false, got)
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
