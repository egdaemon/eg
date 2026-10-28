package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/stretchr/testify/require"
)

func count(t *testing.T, db *sql.DB) (n int, err error) {
	err = db.QueryRowContext(t.Context(), `SELECT count(*) FROM lan_peers`).Scan(&n)
	return n, err
}

func insert(t *testing.T, db *sql.DB, id string) error {
	_, err := db.ExecContext(t.Context(), `INSERT INTO lan_peers (peer_id) VALUES ($1)`, id)
	return err
}

func TestOpenLocked(t *testing.T) {
	t.Run("processes share the file", func(t *testing.T) {
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		path := filepath.Join(t.TempDir(), "shared.db")

		owner, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)
		defer owner.Close()

		sharer, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)
		defer sharer.Close()

		require.NoError(t, insert(t, owner, "a"))
		require.NoError(t, insert(t, sharer, "b"))

		for _, db := range []*sql.DB{owner, sharer} {
			n, err := count(t, db)
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

		require.NoError(t, insert(t, owner, "a"))
		_, err = count(t, sharer) // the sharer now holds a connection through the owner.
		require.NoError(t, err)

		require.NoError(t, owner.Close())

		n, err := count(t, sharer)
		require.NoError(t, err)
		require.Equal(t, 1, n)

		// the sharer owns the file now so the next process goes through it.
		next, err := migrations.OpenLocked(t.Context(), path)
		require.NoError(t, err)
		defer next.Close()
		require.NoError(t, insert(t, next, "b"))
		n, err = count(t, sharer)
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
			wg.Add(1)
			go func() {
				defer wg.Done()
				dbs[i], errs[i] = migrations.OpenLocked(t.Context(), path)
			}()
		}
		wg.Wait()

		for i := range n {
			require.NoError(t, errs[i])
			defer dbs[i].Close()
			require.NoError(t, insert(t, dbs[i], string(rune('a'+i))))
		}

		got, err := count(t, dbs[0])
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
