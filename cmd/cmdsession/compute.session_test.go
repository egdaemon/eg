package cmdsession_test

import (
	"testing"

	"github.com/egdaemon/eg/cmd/cmddb"
	"github.com/egdaemon/eg/cmd/cmdsession"
	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/computesess"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestComputeSessionNew(t *testing.T) {
	t.Run("registers when a session was requested", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		id := uuidx.NewV7()

		_, done := cmdsession.ComputeSession{Session: id}.New(&cmddb.CLIDB{DB: db}).Register(t.Context(), id, "m", "/repo")
		defer done(nil)

		found, err := computesess.Lookup(t.Context(), db, id.String())
		require.NoError(t, err)
		require.True(t, found.Running())
	})

	t.Run("registers nothing without a session", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		_, done := cmdsession.ComputeSession{}.New(&cmddb.CLIDB{DB: db}).Register(t.Context(), uuidx.NewV7(), "m", "/repo")
		defer done(nil)

		sessions, err := computesess.List(t.Context(), db)
		require.NoError(t, err)
		require.Empty(t, sessions)
	})

	t.Run("registers nothing when the database is unavailable", func(t *testing.T) {
		_, done := cmdsession.ComputeSession{Session: uuidx.NewV7()}.New(&cmddb.CLIDB{}).Register(t.Context(), uuidx.NewV7(), "m", "/repo")
		done(nil)
	})
}
