package sqlx_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/stretchr/testify/require"
)

func TestDiscarded(t *testing.T) {
	db := sqltestx.DuckDB(t)

	c, err := sqlx.Discarded(sqlx.Scan(sqlx.NewRowsScanner[int](db.QueryContext(t.Context(), `SELECT * FROM range(5)`))))
	require.NoError(t, err)
	require.EqualValues(t, 5, c)
}
