package migrations_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestOpenLocked(t *testing.T) {
	t.Run("serves the database over postgres while open", func(t *testing.T) {
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("CACHE_DIRECTORY", t.TempDir())

		db, err := migrations.OpenLocked(t.Context(), migrations.Path(migrations.CLI))
		require.NoError(t, err)

		_, err = db.ExecContext(t.Context(), `INSERT INTO serve_sessions (id, module, ssh_port) VALUES ($1, $2, $3)`, "a", "m", 2299)
		require.NoError(t, err)

		socket := migrations.PostgresSocket()
		require.Equal(t, filepath.Join(os.Getenv("RUNTIME_DIRECTORY"), "eg", ".s.PGSQL.5432"), socket)

		require.Eventually(t, func() bool {
			_, err := os.Stat(socket)
			return err == nil
		}, 5*time.Second, 10*time.Millisecond, "the postgres socket must exist while the database is open")

		cfg, err := pgx.ParseConfig(fmt.Sprintf("host=%s port=5432 database=eg sslmode=disable", filepath.Dir(socket)))
		require.NoError(t, err)

		conn, err := pgx.ConnectConfig(t.Context(), cfg)
		require.NoError(t, err)

		var (
			module string
			port   int64
		)
		require.NoError(t, conn.QueryRow(t.Context(), `SELECT module, ssh_port FROM serve_sessions WHERE id = $1`, "a").Scan(&module, &port))
		require.Equal(t, "m", module)
		require.Equal(t, int64(2299), port)
		require.NoError(t, conn.Close(t.Context()))

		require.NoError(t, db.Close())
		_, err = os.Stat(socket)
		require.ErrorIs(t, err, os.ErrNotExist, "the postgres socket must be removed when the database closes")
	})

	t.Run("processes share the file", func(t *testing.T) {
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		path := filepath.Join(t.TempDir(), "shared.db")

		owner, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)
		defer owner.Close()

		sharer, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)
		defer sharer.Close()

		_, err = owner.ExecContext(t.Context(), `INSERT INTO lan_peers (peer_id) VALUES ($1)`, "a")
		require.NoError(t, err)
		_, err = sharer.ExecContext(t.Context(), `INSERT INTO lan_peers (peer_id) VALUES ($1)`, "b")
		require.NoError(t, err)

		for _, db := range []*sql.DB{owner, sharer} {
			var n int
			err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM lan_peers`).Scan(&n)
			require.NoError(t, err)
			require.Equal(t, 2, n)
		}
	})
}
