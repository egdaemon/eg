package history_test

import (
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/history"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestEntryUpsert(t *testing.T) {
	t.Run("records the outcome of a workload", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		e := langx.Clone(history.Entry{}, history.EntryOptionTestDefaults)
		require.NoError(t, history.EntryUpsert(t.Context(), db, e).Scan(&history.Entry{}))

		var found history.Entry
		require.NoError(t, history.EntryFindByID(t.Context(), db, e.ID).Scan(&found))
		require.Equal(t, e.Description, found.Description)
		require.Equal(t, history.ResultCompleted, found.Result)
		require.Equal(t, e.Duration, found.Duration)
	})

	t.Run("replaces the earlier outcome of the same workload", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		e := langx.Clone(history.Entry{}, history.EntryOptionTestDefaults, func(e *history.Entry) { e.Result = history.ResultFailure })
		require.NoError(t, history.EntryUpsert(t.Context(), db, e).Scan(&history.Entry{}))
		e.Result = history.ResultCompleted
		require.NoError(t, history.EntryUpsert(t.Context(), db, e).Scan(&history.Entry{}))

		var found history.Entry
		require.NoError(t, history.EntryFindByID(t.Context(), db, e.ID).Scan(&found))
		require.Equal(t, history.ResultCompleted, found.Result)
	})
}
