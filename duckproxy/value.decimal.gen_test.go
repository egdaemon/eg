package duckproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueDecimal(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		// duckproxy itself never constructs a duckdb.Decimal (that would
		// pull in duckdb-go's cgo bindings) -- only duckproxyserver, which
		// has the real connection, does. Here we only need to confirm the
		// wire decode side: a DecimalValue comes back as its already
		// formatted string, unchanged.
		pv := &Value{Kind: &Value_DecimalValue{DecimalValue: "123.45"}}

		got, err := FromProtoValue(pv)
		require.NoError(t, err)
		require.Equal(t, "123.45", got)
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
