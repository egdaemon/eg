package duckproxy

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueInt(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"int8", int8(7), 7},
		{"int16", int16(1234), 1234},
		{"int32", int32(123456), 123456},
		{"int64", int64(123456789012), 123456789012},
		{"int", int(42), 42},
		{"uint8", uint8(200), 200},
		{"uint16", uint16(60000), 60000},
		{"uint32", uint32(4000000000), 4000000000},
		{"uint64_within_int64_range", uint64(9000000000000000000), 9000000000000000000},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pv, err := toProtoValue(c.in)
			require.NoError(t, err)
			require.Equal(t, c.want, pv.GetIntValue())

			got, err := fromProtoValue(pv)
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}

	t.Run("uint64_beyond_int64_range_falls_back_to_bignum", func(t *testing.T) {
		in := uint64(math.MaxUint64)
		want := new(big.Int).SetUint64(in)

		pv, err := toProtoValue(in)
		require.NoError(t, err)
		require.Equal(t, want.String(), pv.GetBignumValue())

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, want, got)
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
