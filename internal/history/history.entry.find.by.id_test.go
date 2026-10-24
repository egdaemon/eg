package history_test

import (
	"database/sql"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/history"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestEntryFindByID(t *testing.T) {
	t.Run("missing workloads are not found", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		require.ErrorIs(t, history.EntryFindByID(t.Context(), db, "missing").Scan(&history.Entry{}), sql.ErrNoRows)
	})

	t.Run("expired outcomes are not found before they are purged", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		e := langx.Clone(history.Entry{}, history.EntryOptionTestDefaults)
		require.NoError(t, history.EntryUpsert(t.Context(), db, e).Scan(&history.Entry{}))
		sqltestx.Exec(t, db, `UPDATE compute_history SET expires_at = now() - INTERVAL '1 minute'`)

		require.ErrorIs(t, history.EntryFindByID(t.Context(), db, e.ID).Scan(&history.Entry{}), sql.ErrNoRows)
	})
}
