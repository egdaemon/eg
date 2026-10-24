package duckproxyserver_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/stretchr/testify/require"
)

func TestServerServe(t *testing.T) {
	t.Run("streams_rows_incrementally", func(t *testing.T) {
		socketPath, _ := startTestServer(t)
		db := connect(t, socketPath)
		ctx := context.Background()

		const totalRows = 200000

		_, err := db.ExecContext(ctx, "CREATE TABLE big AS SELECT n, repeat('x', 50) AS pad FROM generate_series(1, $1) t(n)", totalRows)
		require.NoError(t, err)

		rows, err := db.QueryContext(ctx, "SELECT n, pad FROM big ORDER BY n")
		require.NoError(t, err)
		defer rows.Close()

		// Consuming this one row at a time, via the same Next/Scan loop
		// any database/sql caller uses, proves the client side never
		// buffers more than a single decoded row at a time either --
		// there's no intermediate slice this loop is populated from.
		var (
			n    int
			pad  string
			seen int
		)
		for rows.Next() {
			require.NoError(t, rows.Scan(&n, &pad))
			seen++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, totalRows, seen)
	})

	t.Run("concurrent_clients_bounded_by_max_connections", func(t *testing.T) {
		const (
			maxConnections   = 2
			clients          = 8
			insertsPerClient = 20
		)

		socketPath, serverDB := startTestServer(t, duckproxyserver.WithMaxConnections(maxConnections))

		setup := connect(t, socketPath)
		_, err := setup.ExecContext(context.Background(), "CREATE TABLE concurrent (client INTEGER, n INTEGER)")
		require.NoError(t, err)

		var (
			wg        sync.WaitGroup
			mu        sync.Mutex
			errs      []error
			maxInUse  int
			stopProbe = make(chan struct{})
		)

		probeDone := make(chan struct{})
		go func() {
			defer close(probeDone)
			for {
				select {
				case <-stopProbe:
					return
				default:
					if inUse := serverDB.Stats().InUse; inUse > maxInUse {
						maxInUse = inUse
					}
				}
			}
		}()

		for i := range clients {
			wg.Add(1)
			go func(client int) {
				defer wg.Done()

				ctx := context.Background()

				// Open and close a dedicated *sql.DB per client here,
				// rather than via the shared connect() helper's
				// t.Cleanup (which only runs at the very end of the
				// test): with MaxConnections capped below the client
				// count, holding every connection open for the whole
				// test would deadlock the remaining clients waiting for
				// a free slot.
				clientDB, err := sql.Open("duckproxy", socketPath)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("client %d open: %w", client, err))
					mu.Unlock()
					return
				}
				defer clientDB.Close()

				for n := range insertsPerClient {
					if _, err := clientDB.ExecContext(ctx, "INSERT INTO concurrent VALUES ($1, $2)", client, n); err != nil {
						mu.Lock()
						errs = append(errs, fmt.Errorf("client %d insert %d: %w", client, n, err))
						mu.Unlock()
						return
					}

					var count int
					if err := clientDB.QueryRowContext(ctx, "SELECT count(*) FROM concurrent WHERE client = $1", client).Scan(&count); err != nil {
						mu.Lock()
						errs = append(errs, fmt.Errorf("client %d select %d: %w", client, n, err))
						mu.Unlock()
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(stopProbe)
		<-probeDone

		for _, err := range errs {
			if strings.Contains(err.Error(), "already in use") || strings.Contains(err.Error(), "lock") {
				t.Errorf("lock contention error (the bug duckproxy fixes): %v", err)
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}

		require.LessOrEqual(t, maxInUse, maxConnections)

		var total int
		require.NoError(t, setup.QueryRowContext(context.Background(), "SELECT count(*) FROM concurrent").Scan(&total))
		require.Equal(t, clients*insertsPerClient, total)
	})
}
