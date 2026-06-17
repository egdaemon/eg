package duckproxy_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/duckproxy"
	"github.com/jackc/pgx/v5"
)

// TestConcurrentClients exercises the actual bug duckproxy fixes: many
// separate client connections hitting the same DuckDB file at once, which
// would otherwise collide on DuckDB's single-process file lock. It also
// checks that WithMaxConnections bounds how many DuckDB connections are
// concurrently in use without dropping any client.
func TestConcurrentClients(t *testing.T) {
	const (
		maxConnections   = 2
		clients          = 8
		insertsPerClient = 20
	)

	dir, db := startTestServer(t, duckproxy.WithMaxConnections(maxConnections))

	setup := connect(t, dir, false)
	if _, err := setup.Exec(context.Background(), "CREATE TABLE concurrent (client INTEGER, n INTEGER)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		errs      []error
		maxInUse  int
		stopProbe = make(chan struct{})
		probeWG   sync.WaitGroup
	)

	probeWG.Go(func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopProbe:
				return
			case <-ticker.C:
				if inUse := db.Stats().InUse; inUse > maxInUse {
					maxInUse = inUse
				}
			}
		}
	})

	for i := range clients {
		wg.Add(1)
		go func(client int) {
			defer wg.Done()

			ctx := context.Background()

			// Connect and close per-client here rather than via the
			// shared connect() helper's t.Cleanup, which only runs at
			// the very end of the test: with MaxConnections capped
			// below the client count, holding every connection open
			// for the whole test would deadlock the remaining clients
			// waiting for a free slot.
			cfg, err := pgx.ParseConfig(fmt.Sprintf("host=%s port=5432 database=test sslmode=disable", dir))
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("client %d parse config: %w", client, err))
				mu.Unlock()
				return
			}
			conn, err := pgx.ConnectConfig(ctx, cfg)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("client %d connect: %w", client, err))
				mu.Unlock()
				return
			}
			defer conn.Close(ctx)

			for n := range insertsPerClient {
				if _, err := conn.Exec(ctx, "INSERT INTO concurrent VALUES ($1, $2)", client, n); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("client %d insert %d: %w", client, n, err))
					mu.Unlock()
					return
				}

				var count int
				if err := conn.QueryRow(ctx, "SELECT count(*) FROM concurrent WHERE client = $1", client).Scan(&count); err != nil {
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
	probeWG.Wait()

	for _, err := range errs {
		if strings.Contains(err.Error(), "already in use") || strings.Contains(err.Error(), "lock") {
			t.Errorf("lock contention error (the bug duckproxy fixes): %v", err)
		} else {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if maxInUse > maxConnections {
		t.Errorf("observed %d DuckDB connections concurrently in use, want at most %d", maxInUse, maxConnections)
	}

	var total int
	if err := setup.QueryRow(context.Background(), "SELECT count(*) FROM concurrent").Scan(&total); err != nil {
		t.Fatalf("final count: %v", err)
	}
	if want := clients * insertsPerClient; total != want {
		t.Errorf("got %d rows, want %d", total, want)
	}
}
