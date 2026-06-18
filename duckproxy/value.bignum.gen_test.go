package duckproxy

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueBignum(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		in, ok := new(big.Int).SetString("170141183460469231731687303715884105727", 10) // > math.MaxInt64
		require.True(t, ok, "failed to construct test big.Int")

		pv, err := toProtoValue(in)
		require.NoError(t, err)
		require.Equal(t, in.String(), pv.GetBignumValue())

		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, in, got)
	})

	t.Run("malformed", func(t *testing.T) {
		pv := &Value{Kind: &Value_BignumValue{BignumValue: "not-a-number"}}
		_, err := fromProtoValue(pv)
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
