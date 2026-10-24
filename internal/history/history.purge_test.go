package history_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/history"
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

		old := langx.Clone(history.Entry{}, history.EntryOptionTestDefaults, func(e *history.Entry) { e.LogsPath = logs })
		keep := langx.Clone(history.Entry{}, history.EntryOptionTestDefaults)
		require.NoError(t, history.EntryUpsert(t.Context(), db, old).Scan(&history.Entry{}))
		require.NoError(t, history.EntryUpsert(t.Context(), db, keep).Scan(&history.Entry{}))
		sqltestx.Exec(t, db, `UPDATE compute_history SET expires_at = now() - INTERVAL '1 minute' WHERE id = ?`, old.ID)

		require.NoError(t, history.Purge(t.Context(), db))

		require.ErrorIs(t, history.EntryFindByID(t.Context(), db, old.ID).Scan(&history.Entry{}), sql.ErrNoRows)
		require.NoError(t, history.EntryFindByID(t.Context(), db, keep.ID).Scan(&history.Entry{}))
		_, err := os.Stat(logs)
		require.True(t, errors.Is(err, os.ErrNotExist))
	})
}
