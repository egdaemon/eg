package computex_test

import (
	"database/sql"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestEntryFindByID(t *testing.T) {
	t.Run("missing workloads are not found", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		require.ErrorIs(t, computex.EntryFindByID(t.Context(), db, "missing").Scan(&computex.Entry{}), sql.ErrNoRows)
	})

	t.Run("expired outcomes are not found before they are purged", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		e := langx.Clone(computex.Entry{}, computex.EntryOptionTestDefaults)
		require.NoError(t, computex.EntryUpsert(t.Context(), db, e).Scan(&computex.Entry{}))
		sqltestx.Exec(t, db, `UPDATE compute_history SET expires_at = now() - INTERVAL '1 minute'`)

		require.ErrorIs(t, computex.EntryFindByID(t.Context(), db, e.ID).Scan(&computex.Entry{}), sql.ErrNoRows)
	})
}
