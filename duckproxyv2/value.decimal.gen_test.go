package duckproxyv2

import (
	"math/big"
	"testing"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"
)

func TestValueDecimal(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		in := duckdb.Decimal{Width: 10, Scale: 2, Value: big.NewInt(12345)}

		pv, err := toProtoValue(in)
		require.NoError(t, err)
		require.Equal(t, in.String(), pv.GetDecimalValue())

		// fromProtoValue intentionally returns the string form, not a
		// reconstructed duckdb.Decimal -- DuckDB re-parses/casts it if
		// it's ever bound back as a parameter.
		got, err := fromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, in.String(), got)
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
