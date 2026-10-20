package sqlx_test

import (
	"database/sql"
	"testing"

	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/stretchr/testify/require"
)

func TestScanOne(t *testing.T) {
	db := sqltestx.DuckDB(t)

	t.Run("returns the first row", func(t *testing.T) {
		v, err := sqlx.ScanOne(sqlx.NewRowsScanner[int](db.QueryContext(t.Context(), `SELECT 7`)))
		require.NoError(t, err)
		require.Equal(t, 7, v)
	})

	t.Run("no rows reports sql.ErrNoRows", func(t *testing.T) {
		_, err := sqlx.ScanOne(sqlx.NewRowsScanner[int](db.QueryContext(t.Context(), `SELECT * FROM range(0)`)))
		require.ErrorIs(t, err, sql.ErrNoRows)
	})
}
