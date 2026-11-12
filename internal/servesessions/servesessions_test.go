package servesessions_test

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/servesessions"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/stretchr/testify/require"
)

func TestRegisterListStop(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, migrations.Migrate(t.Context(), db))

	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "a", Module: "m", Directory: "/repo", SSHRequested: 0, SSHPort: 2299, SSHUser: "egd"}))

	sessions, err := servesessions.List(t.Context(), db)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.True(t, sessions[0].Running())
	require.Equal(t, int64(2299), sessions[0].SSHPort)
	require.Equal(t, int64(0), sessions[0].SSHRequested)
	require.Equal(t, "/repo", sessions[0].Directory)
	require.Equal(t, int64(os.Getpid()), sessions[0].PID)

	require.NoError(t, servesessions.Stop(t.Context(), db, "a"))

	// stopped sessions are kept as history with what is needed to restart them, the port is no longer published.
	sessions, err = servesessions.List(t.Context(), db)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.False(t, sessions[0].Running())
	require.NotNil(t, sessions[0].StoppedAt)
	require.Equal(t, int64(0), sessions[0].SSHPort)
	require.Equal(t, "/repo", sessions[0].Directory)
	require.Equal(t, "m", sessions[0].Module)
}

func TestListOrdersRunningFirst(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, migrations.Migrate(t.Context(), db))

	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "old", SSHPort: 1}))
	require.NoError(t, servesessions.Stop(t.Context(), db, "old"))
	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "live", SSHPort: 2}))

	sessions, err := servesessions.List(t.Context(), db)
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	require.Equal(t, "live", sessions[0].ID)
	require.Equal(t, "old", sessions[1].ID)
}

func TestRegisterRestartsStoppedSession(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, migrations.Migrate(t.Context(), db))

	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "a", Module: "m", SSHPort: 2299}))
	require.NoError(t, servesessions.Stop(t.Context(), db, "a"))
	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "a", Module: "m", SSHPort: 2300}))

	sessions, err := servesessions.List(t.Context(), db)
	require.NoError(t, err)
	require.Len(t, sessions, 1, "restarting must not duplicate the session")
	require.True(t, sessions[0].Running())
	require.Equal(t, int64(2300), sessions[0].SSHPort)
}

func TestRegisterSameRunningProcessUpdatesPort(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, migrations.Migrate(t.Context(), db))

	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "a", SSHPort: 2299}))
	first, err := servesessions.Lookup(t.Context(), db, "a")
	require.NoError(t, err)

	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "a", SSHPort: 2300}))
	second, err := servesessions.Lookup(t.Context(), db, "a")
	require.NoError(t, err)

	require.Equal(t, int64(2300), second.SSHPort)
	require.Equal(t, first.StartedAt, second.StartedAt, "a new container for the same run is not a restart")
}

func TestLookup(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, migrations.Migrate(t.Context(), db))

	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "a", SSHPort: 2299}))
	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "b", SSHPort: 2300}))
	require.NoError(t, servesessions.Stop(t.Context(), db, "b"))

	s, err := servesessions.Lookup(t.Context(), db, "a")
	require.NoError(t, err)
	require.Equal(t, int64(2299), s.SSHPort)

	s, err = servesessions.Lookup(t.Context(), db, "b")
	require.NoError(t, err)
	require.False(t, s.Running(), "stopped sessions can be looked up")

	_, err = servesessions.Lookup(t.Context(), db, "missing")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestRegisterRejectsIDOfRunningSessionInAnotherProcess(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, migrations.Migrate(t.Context(), db))

	// the parent of the test process is alive and not us.
	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "a", PID: int64(os.Getppid()), SSHPort: 2299}))
	require.Error(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "a", SSHPort: 2300}))

	s, err := servesessions.Lookup(t.Context(), db, "a")
	require.NoError(t, err)
	require.Equal(t, int64(2299), s.SSHPort)
}

func TestDeadProcessesAreStopped(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, migrations.Migrate(t.Context(), db))

	// pid beyond pid_max never exists.
	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "dead", PID: 1 << 30, SSHPort: 2222}))

	sessions, err := servesessions.List(t.Context(), db)
	require.NoError(t, err)
	require.Len(t, sessions, 1, "a session that was killed is history, not forgotten")
	require.False(t, sessions[0].Running())
	require.Equal(t, int64(0), sessions[0].SSHPort)

	// and its id is free to restart in another process.
	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "dead", PID: int64(os.Getppid()), SSHPort: 2223}))
}

func TestHistoryIsLimited(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, migrations.Migrate(t.Context(), db))

	for i := 0; i < servesessions.HistoryLimit+5; i++ {
		id := fmt.Sprintf("s%03d", i)
		require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: id, SSHPort: 2222}))
		require.NoError(t, servesessions.Stop(t.Context(), db, id))
	}
	// pruning happens as sessions are registered.
	require.NoError(t, servesessions.Register(t.Context(), db, servesessions.Session{ID: "live", SSHPort: 2222}))

	sessions, err := servesessions.List(t.Context(), db)
	require.NoError(t, err)
	require.Len(t, sessions, servesessions.HistoryLimit+1)
	require.Equal(t, "live", sessions[0].ID)

	_, err = servesessions.Lookup(t.Context(), db, "s000")
	require.ErrorIs(t, err, sql.ErrNoRows, "the oldest stopped sessions are forgotten")
}
