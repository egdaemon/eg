package goosex

import (
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"
)

func TestUp(t *testing.T) {
	t.Run("concurrent callers apply each migration once", func(t *testing.T) {
		// the first migration is applied up front so goose's own, retried, handling of concurrently creating the
		// version table doesn't serialize the callers. the rest take long enough to overlap and alter the same
		// tables, which duckdb aborts as a write-write conflict when callers migrate at the same time.
		initial := fstest.MapFS{
			"20261020000001_init.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE init (id INTEGER PRIMARY KEY);\n-- +goose Down\nDROP TABLE init;\n")},
		}
		migrations := fstest.MapFS{
			"20261020000001_init.sql": initial["20261020000001_init.sql"],
			"20261020000002_a.sql":    &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE a AS SELECT range AS id FROM range(2000000);\n-- +goose Down\nDROP TABLE a;\n")},
			"20261020000003_b.sql":    &fstest.MapFile{Data: []byte("-- +goose Up\nALTER TABLE a ADD COLUMN b INTEGER;\n-- +goose Down\nALTER TABLE a DROP COLUMN b;\n")},
			"20261020000004_c.sql":    &fstest.MapFile{Data: []byte("-- +goose Up\nDROP TABLE IF EXISTS c;\nCREATE TABLE c AS SELECT range AS id FROM range(2000000);\n-- +goose Down\nDROP TABLE c;\n")},
		}

		for range 5 {
			db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "migrate.db"))
			require.NoError(t, err)
			defer db.Close()

			require.NoError(t, Up(t.Context(), db, initial))

			const n = 8
			errs := make([]error, n)
			var wg sync.WaitGroup
			for i := range n {
				wg.Go(func() {
					errs[i] = Up(t.Context(), db, migrations)
				})
			}
			wg.Wait()

			for i := range n {
				require.NoError(t, errs[i])
			}

			var applied int
			require.NoError(t, db.QueryRowContext(t.Context(), `SELECT count(*) FROM goose_db_version WHERE version_id > 0 AND is_applied`).Scan(&applied))
			require.Equal(t, 4, applied)
		}
	})
}

func TestDuckdbLocker(t *testing.T) {
	t.Run("a held lock excludes others until it is released", func(t *testing.T) {
		db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "lock.db"))
		require.NoError(t, err)
		defer db.Close()

		a, b := newDuckdbLocker(), newDuckdbLocker()
		require.NoError(t, a.Lock(t.Context(), db))

		held, err := b.try(t.Context(), db)
		require.NoError(t, err)
		require.False(t, held)

		require.NoError(t, a.Unlock(t.Context(), db))

		held, err = b.try(t.Context(), db)
		require.NoError(t, err)
		require.True(t, held)
	})

	t.Run("waiting for a held lock acquires it once released", func(t *testing.T) {
		db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "lock.db"))
		require.NoError(t, err)
		defer db.Close()

		a, b := newDuckdbLocker(), newDuckdbLocker()
		require.NoError(t, a.Lock(t.Context(), db))

		acquired := make(chan error, 1)
		go func() { acquired <- b.Lock(t.Context(), db) }()

		select {
		case err := <-acquired:
			t.Fatalf("acquired a held lock: %v", err)
		case <-time.After(200 * time.Millisecond):
		}

		require.NoError(t, a.Unlock(t.Context(), db))
		require.NoError(t, <-acquired)
	})

	t.Run("a holder that never released is taken over after its lease", func(t *testing.T) {
		db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "lock.db"))
		require.NoError(t, err)
		defer db.Close()

		dead, next := newDuckdbLocker(), newDuckdbLocker()
		dead.lease = 100 * time.Millisecond
		require.NoError(t, dead.Lock(t.Context(), db))

		start := time.Now()
		require.NoError(t, next.Lock(t.Context(), db))
		require.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond, "the lease was not waited out")
	})

	t.Run("releasing a lock that was taken over leaves the new holder", func(t *testing.T) {
		db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "lock.db"))
		require.NoError(t, err)
		defer db.Close()

		slow, next, other := newDuckdbLocker(), newDuckdbLocker(), newDuckdbLocker()
		slow.lease = 20 * time.Millisecond
		require.NoError(t, slow.Lock(t.Context(), db))
		time.Sleep(50 * time.Millisecond)
		require.NoError(t, next.Lock(t.Context(), db))

		// slow finishing late must not release next's lock.
		require.NoError(t, slow.Unlock(t.Context(), db))

		held, err := other.try(t.Context(), db)
		require.NoError(t, err)
		require.False(t, held)
	})

	t.Run("gives up when the lock stays held", func(t *testing.T) {
		db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "lock.db"))
		require.NoError(t, err)
		defer db.Close()

		a, b := newDuckdbLocker(), newDuckdbLocker()
		b.wait = 100 * time.Millisecond
		require.NoError(t, a.Lock(t.Context(), db))
		require.Error(t, b.Lock(t.Context(), db))
	})
}
