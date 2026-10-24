package duckproxyserver

import (
	"math/big"
	"testing"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"
)

func TestToProtoValueInterval(t *testing.T) {
	in := duckdb.Interval{Months: 2, Days: 10, Micros: 123456}

	pv, err := toProtoValue(in)
	require.NoError(t, err)
	iv := pv.GetIntervalValue()
	require.NotNil(t, iv)
	require.Equal(t, in.Months, iv.GetMonths())
	require.Equal(t, in.Days, iv.GetDays())
	require.Equal(t, in.Micros, iv.GetMicros())

	got, err := fromProtoValue(pv)
	require.NoError(t, err)
	require.Equal(t, in, got)
}

func TestToProtoValueDecimal(t *testing.T) {
	in := duckdb.Decimal{Width: 10, Scale: 2, Value: big.NewInt(12345)}

	pv, err := toProtoValue(in)
	require.NoError(t, err)
	require.Equal(t, in.String(), pv.GetDecimalValue())

	// fromProtoValue intentionally returns the string form, not a
	// reconstructed duckdb.Decimal -- DuckDB re-parses/casts it if it's
	// ever bound back as a parameter.
	got, err := fromProtoValue(pv)
	require.NoError(t, err)
	require.Equal(t, in.String(), got)
}
