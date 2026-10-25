package computex_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestPurge(t *testing.T) {
	t.Run("removes expired outcomes and their logs", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		logs := filepath.Join(t.TempDir(), "old.log")
		require.NoError(t, os.WriteFile(logs, []byte("old"), 0600))

		old := langx.Clone(computex.Entry{}, computex.EntryOptionTestDefaults, func(e *computex.Entry) { e.LogsPath = logs })
		keep := langx.Clone(computex.Entry{}, computex.EntryOptionTestDefaults)
		require.NoError(t, computex.EntryUpsert(t.Context(), db, old).Scan(&computex.Entry{}))
		require.NoError(t, computex.EntryUpsert(t.Context(), db, keep).Scan(&computex.Entry{}))
		sqltestx.Exec(t, db, `UPDATE compute_history SET expires_at = now() - INTERVAL '1 minute' WHERE id = ?`, old.ID)

		require.NoError(t, computex.Purge(t.Context(), db))

		require.ErrorIs(t, computex.EntryFindByID(t.Context(), db, old.ID).Scan(&computex.Entry{}), sql.ErrNoRows)
		require.NoError(t, computex.EntryFindByID(t.Context(), db, keep.ID).Scan(&computex.Entry{}))
		_, err := os.Stat(logs)
		require.True(t, errors.Is(err, os.ErrNotExist))
	})
}
