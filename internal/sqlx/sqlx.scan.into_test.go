package sqlx_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/stretchr/testify/require"
)

func TestScanInto(t *testing.T) {
	db := sqltestx.DuckDB(t)

	t.Run("collects every row", func(t *testing.T) {
		var got []int
		require.NoError(t, sqlx.ScanInto(sqlx.NewRowsScanner[int](db.QueryContext(t.Context(), `SELECT * FROM range(3)`)), &got))
		require.Equal(t, []int{0, 1, 2}, got)
	})

	t.Run("no rows is not an error", func(t *testing.T) {
		var got []int
		require.NoError(t, sqlx.ScanInto(sqlx.NewRowsScanner[int](db.QueryContext(t.Context(), `SELECT * FROM range(0)`)), &got))
		require.Empty(t, got)
	})

	t.Run("query errors are surfaced", func(t *testing.T) {
		var got []int
		require.Error(t, sqlx.ScanInto(sqlx.NewRowsScanner[int](db.QueryContext(t.Context(), `SELECT * FROM missing`)), &got))
	})
}
