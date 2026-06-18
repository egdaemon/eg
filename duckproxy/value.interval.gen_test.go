package duckproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueInterval(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		in := &Interval{Months: 2, Days: 10, Micros: 123456}

		pv, err := ToProtoValue(in)
		require.NoError(t, err)
		iv := pv.GetIntervalValue()
		require.NotNil(t, iv)
		require.Equal(t, in.Months, iv.GetMonths())
		require.Equal(t, in.Days, iv.GetDays())
		require.Equal(t, in.Micros, iv.GetMicros())

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
