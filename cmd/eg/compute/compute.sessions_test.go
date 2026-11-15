package compute

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/migrations"
	egcompute "github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computesess"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/jsonx"
	"github.com/stretchr/testify/require"
)

func TestWriteSessions(t *testing.T) {
	t.Run("jsonl is one session per line, running first", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "stopped", Module: "m", Directory: "/repo", SSHRequested: 2222, SSHPort: 2222, SSHUser: "egd", SSHKeyPath: "/k"}))
		require.NoError(t, computesess.Stop(t.Context(), db, "stopped"))
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "live", Module: "", Directory: "/repo", SSHRequested: 0, SSHPort: 44999, SSHUser: "egd", SSHKeyPath: "/k"}))

		var out bytes.Buffer
		require.NoError(t, writeSessions(t.Context(), db, &out, true))

		// one session per line.
		lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
		require.Len(t, lines, 2)

		decoded := make([]*egcompute.ServeSession, 0, len(lines))
		for _, line := range lines {
			var s egcompute.ServeSession
			require.NoError(t, jsonx.Unmarshal([]byte(line), &s))
			decoded = append(decoded, &s)
		}

		// running first, then the history.
		require.Equal(t, "live", decoded[0].Id)
		require.Equal(t, int64(44999), decoded[0].SshPort)
		require.Equal(t, int64(0), decoded[0].SshRequested)
		require.Empty(t, decoded[0].StoppedAt, "running sessions have not stopped")
		require.NotEmpty(t, decoded[0].StartedAt)

		require.Equal(t, "stopped", decoded[1].Id)
		require.Equal(t, "/repo", decoded[1].Directory)
		require.Equal(t, int64(2222), decoded[1].SshRequested)
		require.Equal(t, int64(0), decoded[1].SshPort, "the port is no longer published once stopped")
		require.NotEmpty(t, decoded[1].StoppedAt)
		_, err := time.Parse(time.RFC3339Nano, decoded[1].StoppedAt)
		require.NoError(t, err)
	})

	t.Run("jsonl is empty without sessions", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		var out bytes.Buffer
		require.NoError(t, writeSessions(t.Context(), db, &out, true))

		require.Empty(t, out.String(), "an empty history is no lines")
	})

	t.Run("table", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "live", Module: "m", Directory: "/repo", SSHPort: 44999}))

		var out bytes.Buffer
		require.NoError(t, writeSessions(t.Context(), db, &out, false))

		require.Contains(t, out.String(), "STATE")
		require.Contains(t, out.String(), "live")
		require.Contains(t, out.String(), "running")
		require.Contains(t, out.String(), "44999")
	})

	t.Run("listing stays the default of sessions, rm is its own command", func(t *testing.T) {
		var cli struct {
			Sessions sessionsCmd `cmd:"" name:"sessions"`
		}

		parser, err := kong.New(&cli)
		require.NoError(t, err)

		for args, expected := range map[string]string{
			"sessions":         "sessions list",
			"sessions --jsonl": "sessions list",
			"sessions rm a b":  "sessions rm <id>",
		} {
			kctx, err := parser.Parse(strings.Fields(args))
			require.NoError(t, err, args)
			require.Equal(t, expected, kctx.Command(), args)
		}

	})

	t.Run("rm forgets stopped sessions and says which", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "old", Module: "m", Directory: "/repo"}))
		require.NoError(t, computesess.Stop(t.Context(), db, "old"))
		require.NoError(t, computesess.Register(t.Context(), db, computesess.Session{ID: "live", Module: "m", Directory: "/repo"}))

		var out bytes.Buffer
		err := removeSessions(t.Context(), db, &out, []string{"old", "live", "missing"})

		// the stopped one is gone and reported, the running and the missing ones are errors and untouched.
		require.Equal(t, "old\n", out.String())
		require.ErrorContains(t, err, "session live is running")
		require.ErrorContains(t, err, "no session missing")

		var listed bytes.Buffer
		require.NoError(t, writeSessions(t.Context(), db, &listed, true))
		require.Contains(t, listed.String(), `"id":"live"`)
		require.NotContains(t, listed.String(), `"id":"old"`)
	})
}
