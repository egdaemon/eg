package computex_test

import (
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestEntryUpsert(t *testing.T) {
	t.Run("records the outcome of a workload", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		e := langx.Clone(computex.Entry{}, computex.EntryOptionTestDefaults)
		require.NoError(t, computex.EntryUpsert(t.Context(), db, e).Scan(&computex.Entry{}))

		var found computex.Entry
		require.NoError(t, computex.EntryFindByID(t.Context(), db, e.ID).Scan(&found))
		require.Equal(t, e.Description, found.Description)
		require.Equal(t, computex.ResultCompleted, found.Result)
		require.Equal(t, e.Duration, found.Duration)
	})

	t.Run("replaces the earlier outcome of the same workload", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		e := langx.Clone(computex.Entry{}, computex.EntryOptionTestDefaults, func(e *computex.Entry) { e.Result = computex.ResultFailure })
		require.NoError(t, computex.EntryUpsert(t.Context(), db, e).Scan(&computex.Entry{}))
		e.Result = computex.ResultCompleted
		require.NoError(t, computex.EntryUpsert(t.Context(), db, e).Scan(&computex.Entry{}))

		var found computex.Entry
		require.NoError(t, computex.EntryFindByID(t.Context(), db, e.ID).Scan(&found))
		require.Equal(t, computex.ResultCompleted, found.Result)
	})
}
