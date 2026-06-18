package duckproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueFloat(t *testing.T) {
	t.Run("float32", func(t *testing.T) {
		pv, err := ToProtoValue(float32(1.5))
		require.NoError(t, err)
		require.Equal(t, float64(1.5), pv.GetDoubleValue())

		got, err := FromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, float64(1.5), got)
	})

	t.Run("float64", func(t *testing.T) {
		pv, err := ToProtoValue(float64(3.14159))
		require.NoError(t, err)

		got, err := FromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, float64(3.14159), got)
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
