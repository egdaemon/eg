package computesess_test

import (
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/computesess"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestBuilder(t *testing.T) {
	t.Run("registers a run without ssh and stops it when done", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		id := uuidx.NewV7()

		_, done := computesess.New(db).Agent(true).Patch(true).Register(t.Context(), id, "m", "/repo")

		found, err := computesess.Lookup(t.Context(), db, id.String())
		require.NoError(t, err)
		require.True(t, found.Running())
		require.Equal(t, "m", found.Module)
		require.Equal(t, "/repo", found.Directory)
		require.Equal(t, int64(-1), found.SSHRequested)
		require.False(t, found.CapSSH)
		require.True(t, found.CapAgent)
		require.True(t, found.CapPatch)

		done(nil)

		found, err = computesess.Lookup(t.Context(), db, id.String())
		require.NoError(t, err)
		require.False(t, found.Running())
	})

	t.Run("records the branch", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		id := uuidx.NewV7()

		_, done := computesess.New(db).Branch("eg/mine").Register(t.Context(), id, "m", "/repo")
		defer done(nil)

		found, err := computesess.Lookup(t.Context(), db, id.String())
		require.NoError(t, err)
		require.Equal(t, "eg/mine", found.Branch)
	})

	t.Run("returns the saved branch when none is requested", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		id := uuidx.NewV7()

		_, done := computesess.New(db).Branch("eg/mine").Register(t.Context(), id, "m", "/repo")
		done(nil)

		resumed, done := computesess.New(db).Register(t.Context(), id, "m", "/repo")
		defer done(nil)
		require.Equal(t, "eg/mine", resumed.Branch)

		found, err := computesess.Lookup(t.Context(), db, id.String())
		require.NoError(t, err)
		require.Equal(t, "eg/mine", found.Branch, "registering without a branch keeps the saved one")

		adjusted, done2 := computesess.New(db).Branch("eg/other").Register(t.Context(), id, "m", "/repo")
		defer done2(nil)
		require.Equal(t, "eg/other", adjusted.Branch)
	})

	t.Run("uses the default when none is requested or saved, and saves it", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		id := uuidx.NewV7()

		registered, done := computesess.New(db).Branch("", "eg/default", "eg/other").Register(t.Context(), id, "m", "/repo")
		defer done(nil)
		require.Equal(t, "eg/default", registered.Branch)

		found, err := computesess.Lookup(t.Context(), db, id.String())
		require.NoError(t, err)
		require.Equal(t, "eg/default", found.Branch)
	})

	t.Run("prefers the saved branch to the default but not to the requested one", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		id := uuidx.NewV7()

		_, done := computesess.New(db).Branch("eg/mine", "eg/default").Register(t.Context(), id, "m", "/repo")
		done(nil)

		resumed, done := computesess.New(db).Branch("", "eg/default").Register(t.Context(), id, "m", "/repo")
		done(nil)
		require.Equal(t, "eg/mine", resumed.Branch)

		adjusted, done := computesess.New(db).Branch("eg/adjusted", "eg/default").Register(t.Context(), id, "m", "/repo")
		defer done(nil)
		require.Equal(t, "eg/adjusted", adjusted.Branch)
	})

	t.Run("does nothing without a database", func(t *testing.T) {
		_, done := computesess.New(nil).SSH(0, "key").Register(t.Context(), uuidx.NewV7(), "m", "/repo")
		done(nil)
	})

	t.Run("a run is visible before its ssh port is published, without one, and stops without ever having one", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		id := uuidx.NewV7()

		_, done := computesess.New(db).SSH(0, "key").PTY("/run/user/1000/eg/pty.d/x.sock").Register(t.Context(), id, "m", "/repo")

		found, err := computesess.Lookup(t.Context(), db, id.String())
		require.NoError(t, err)
		require.True(t, found.Running())
		require.Equal(t, int64(0), found.SSHPort)
		require.Equal(t, "/run/user/1000/eg/pty.d/x.sock", found.PtySocket)

		done(nil)

		found, err = computesess.Lookup(t.Context(), db, id.String())
		require.NoError(t, err)
		require.False(t, found.Running())
		require.Equal(t, int64(0), found.SSHPort)
	})
}
