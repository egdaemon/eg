package computesess_test

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/computesess"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/stretchr/testify/require"
)

func TestServeSessions(t *testing.T) {
	t.Run("register, list and stop", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "a", Module: "m", Directory: "/repo", SSHRequested: 0, SSHPort: 2299, SSHUser: "egd"}))

		sessions, err := computesess.List(t.Context(), db)
		require.NoError(t, err)
		require.Len(t, sessions, 1)
		require.True(t, sessions[0].Running())
		require.Equal(t, int64(2299), sessions[0].SSHPort)
		require.Equal(t, int64(0), sessions[0].SSHRequested)
		require.Equal(t, "/repo", sessions[0].Directory)
		require.Equal(t, int64(os.Getpid()), sessions[0].Pid)

		require.NoError(t, computesess.Stop(t.Context(), db, "a"))

		// stopped sessions are kept as history with what is needed to restart them, the port is no longer published.
		sessions, err = computesess.List(t.Context(), db)
		require.NoError(t, err)
		require.Len(t, sessions, 1)
		require.False(t, sessions[0].Running())
		require.NotNil(t, sessions[0].StoppedAt)
		require.Equal(t, int64(0), sessions[0].SSHPort)
		require.Equal(t, "/repo", sessions[0].Directory)
		require.Equal(t, "m", sessions[0].Module)
	})

	t.Run("list orders running first", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "old", SSHPort: 1}))
		require.NoError(t, computesess.Stop(t.Context(), db, "old"))
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "live", SSHPort: 2}))

		sessions, err := computesess.List(t.Context(), db)
		require.NoError(t, err)
		require.Len(t, sessions, 2)
		require.Equal(t, "live", sessions[0].ID)
		require.Equal(t, "old", sessions[1].ID)
	})

	t.Run("register restarts a stopped session", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "a", Module: "m", SSHPort: 2299}))
		require.NoError(t, computesess.Stop(t.Context(), db, "a"))
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "a", Module: "m", SSHPort: 2300}))

		sessions, err := computesess.List(t.Context(), db)
		require.NoError(t, err)
		require.Len(t, sessions, 1, "restarting must not duplicate the session")
		require.True(t, sessions[0].Running())
		require.Equal(t, int64(2300), sessions[0].SSHPort)
	})

	t.Run("register from the same running process updates the port", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "a", SSHPort: 2299}))
		first, err := computesess.Lookup(t.Context(), db, "a")
		require.NoError(t, err)

		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "a", SSHPort: 2300}))
		second, err := computesess.Lookup(t.Context(), db, "a")
		require.NoError(t, err)

		require.Equal(t, int64(2300), second.SSHPort)
		require.Equal(t, first.StartedAt, second.StartedAt, "a new container for the same run is not a restart")
	})

	t.Run("lookup", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "a", SSHPort: 2299}))
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "b", SSHPort: 2300}))
		require.NoError(t, computesess.Stop(t.Context(), db, "b"))

		s, err := computesess.Lookup(t.Context(), db, "a")
		require.NoError(t, err)
		require.Equal(t, int64(2299), s.SSHPort)

		s, err = computesess.Lookup(t.Context(), db, "b")
		require.NoError(t, err)
		require.False(t, s.Running(), "stopped sessions can be looked up")

		_, err = computesess.Lookup(t.Context(), db, "missing")
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("register rejects the id of a running session in another process", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		// the parent of the test process is alive and not us.
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "a", Pid: int64(os.Getppid()), SSHPort: 2299}))
		require.Error(t, computesess.Register(t.Context(), db, computesess.Session{ID: "a", SSHPort: 2300}))

		s, err := computesess.Lookup(t.Context(), db, "a")
		require.NoError(t, err)
		require.Equal(t, int64(2299), s.SSHPort)
	})

	t.Run("dead processes are stopped", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		// pid beyond pid_max never exists.
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "dead", Pid: 1 << 30, SSHPort: 2222}))

		sessions, err := computesess.List(t.Context(), db)
		require.NoError(t, err)
		require.Len(t, sessions, 1, "a session that was killed is history, not forgotten")
		require.False(t, sessions[0].Running())
		require.Equal(t, int64(0), sessions[0].SSHPort)

		// and its id is free to restart in another process.
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "dead", Pid: int64(os.Getppid()), SSHPort: 2223}))
	})

	t.Run("history is limited", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		for i := 0; i < computesess.HistoryLimit+5; i++ {
			id := fmt.Sprintf("s%03d", i)
			require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: id, SSHPort: 2222}))
			require.NoError(t, computesess.Stop(t.Context(), db, id))
		}
		// pruning happens as sessions are registered.
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "live", SSHPort: 2222}))

		sessions, err := computesess.List(t.Context(), db)
		require.NoError(t, err)
		require.Len(t, sessions, computesess.HistoryLimit+1)
		require.Equal(t, "live", sessions[0].ID)

		_, err = computesess.Lookup(t.Context(), db, "s000")
		require.ErrorIs(t, err, sql.ErrNoRows, "the oldest stopped sessions are forgotten")
	})
}
