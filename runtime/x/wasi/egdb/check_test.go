package egdb

import (
	"bytes"
	"database/sql"
	"log"
	"os"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"
)

func logged(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestCheck(t *testing.T) {
	t.Run("reports an unavailable database without failing", func(t *testing.T) {
		t.Setenv("EG_COMPUTE_RUNTIME_DIRECTORY", t.TempDir())
		out := logged(t)

		require.NoError(t, Check(t.Context(), nil))
		require.Contains(t, out.String(), "egdb unavailable:")
		require.Contains(t, out.String(), "egdb cache file")
	})
}

func TestProbe(t *testing.T) {
	t.Run("reports the database it is connected to", func(t *testing.T) {
		db, err := sql.Open("duckdb", "")
		require.NoError(t, err)
		defer db.Close()
		out := logged(t)

		require.NoError(t, probe(t.Context(), db))
		require.Contains(t, out.String(), "egdb available, database:")
	})

	t.Run("reports a database it can not query without failing", func(t *testing.T) {
		db, err := sql.Open("duckdb", "")
		require.NoError(t, err)
		require.NoError(t, db.Close())
		out := logged(t)

		require.NoError(t, probe(t.Context(), db))
		require.Contains(t, out.String(), "egdb connected but unusable:")
	})
}
