package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

	t.Run("recovers when the owner goes away", func(t *testing.T) {
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		path := filepath.Join(t.TempDir(), "shared.db")

		owner, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)

		sharer, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)
		defer sharer.Close()

		_, err = owner.ExecContext(t.Context(), `INSERT INTO lan_peers (peer_id) VALUES ($1)`, "a")
		require.NoError(t, err)
		// the sharer now holds a connection through the owner.
		var n int
		err = sharer.QueryRowContext(t.Context(), `SELECT count(*) FROM lan_peers`).Scan(&n)
		require.NoError(t, err)

		require.NoError(t, owner.Close())

		err = sharer.QueryRowContext(t.Context(), `SELECT count(*) FROM lan_peers`).Scan(&n)
		require.NoError(t, err)
		require.Equal(t, 1, n)

		// the sharer owns the file now so the next process goes through it.
		next, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)
		defer next.Close()
		_, err = next.ExecContext(t.Context(), `INSERT INTO lan_peers (peer_id) VALUES ($1)`, "b")
		require.NoError(t, err)
		err = sharer.QueryRowContext(t.Context(), `SELECT count(*) FROM lan_peers`).Scan(&n)
		require.NoError(t, err)
		require.Equal(t, 2, n)
	})

	t.Run("concurrent opens all succeed", func(t *testing.T) {
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		path := filepath.Join(t.TempDir(), "shared.db")

		const n = 8
		dbs := make([]*sql.DB, n)
		errs := make([]error, n)

		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				dbs[i], errs[i] = migrations.OpenLocked(t.Context(), path)
			})
		}
		wg.Wait()

		for i := range n {
			require.NoError(t, errs[i])
			defer dbs[i].Close()
			_, err := dbs[i].ExecContext(t.Context(), `INSERT INTO lan_peers (peer_id) VALUES ($1)`, string(rune('a'+i)))
			require.NoError(t, err)
		}

		var got int
		err := dbs[0].QueryRowContext(t.Context(), `SELECT count(*) FROM lan_peers`).Scan(&got)
		require.NoError(t, err)
		require.Equal(t, n, got)
	})

	t.Run("replaces a stale socket", func(t *testing.T) {
		runtime := t.TempDir()
		t.Setenv("RUNTIME_DIRECTORY", runtime)
		path := filepath.Join(t.TempDir(), "shared.db")

		first, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)

		entries, err := os.ReadDir(runtime)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		socket := filepath.Join(runtime, entries[0].Name())

		require.NoError(t, first.Close())
		_, err = os.Stat(socket)
		require.ErrorIs(t, err, os.ErrNotExist, "the socket is removed when the owner closes")

		// a crash leaves the socket behind.
		require.NoError(t, os.WriteFile(socket, nil, 0600))
		db, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)
		require.NoError(t, db.Close())
	})

	t.Run("gives up waiting when the context ends", func(t *testing.T) {
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		path := filepath.Join(t.TempDir(), "shared.db")

		ctx, done := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer done()
		<-ctx.Done()

		_, err := migrations.OpenLocked(ctx, path)
		require.Error(t, err)
	})
}
