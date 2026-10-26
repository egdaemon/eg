package duckproxy_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/stretchr/testify/require"
)

const (
	batchRows   = 128
	batchParams = 5
)

// insertReturning issues a single multi-row INSERT ... RETURNING of batchRows rows,
// the same shape as the generated coverage batch insert, and drains the returned rows.
func insertReturning(ctx context.Context, db *sql.DB, offset int) (n int, err error) {
	tuples := make([]string, 0, batchRows)
	args := make([]any, 0, batchRows*batchParams)
	for i := range batchRows {
		o := i * batchParams
		tuples = append(tuples, fmt.Sprintf("($%d,$%d,$%d,DEFAULT,$%d,$%d)", o+1, o+2, o+3, o+4, o+5))
		args = append(args, float64(i), fmt.Sprintf("fn-%d", offset+i), int64(offset+i), fmt.Sprintf("path-%d", offset+i), float64(i))
	}

	query := `INSERT INTO batch ("branches","fnname","hits","id","path","statements") VALUES ` +
		strings.Join(tuples, ",") +
		` RETURNING "branches","fnname","hits","id","path","statements"`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			branches, statements float64
			fnname, path         string
			hits, id             int64
		)
		if err = rows.Scan(&branches, &fnname, &hits, &id, &path, &statements); err != nil {
			return n, err
		}
		n++
	}

	return n, rows.Err()
}

const batchSchema = `CREATE TABLE batch (
	branches DOUBLE,
	fnname VARCHAR,
	hits BIGINT,
	id BIGINT DEFAULT epoch_ms(now()::TIMESTAMP),
	path VARCHAR,
	statements DOUBLE
)`

func TestDriverLargeReturningBatch(t *testing.T) {
	socketPath, _ := startTestServer(t)
	db := connect(t, socketPath)

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	_, err := db.ExecContext(ctx, batchSchema)
	require.NoError(t, err)

	n, err := insertReturning(ctx, db, 0)
	require.NoError(t, err)
	require.Equal(t, batchRows, n)
}

// every report opens a fresh client, runs a batch, and closes. more sequential
// sessions than the server's connection cap must not starve later sessions.
func TestDriverSequentialSessionsRelease(t *testing.T) {
	const sessions = 12

	socketPath, _ := startTestServer(t)

	setup := connect(t, socketPath)
	_, err := setup.ExecContext(context.Background(), batchSchema)
	require.NoError(t, err)

	for i := range sessions {
		t.Run(fmt.Sprintf("session_%d", i), func(t *testing.T) {
			ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()

			db, err := sql.Open("duckproxy", socketPath)
			require.NoError(t, err)

			n, err := insertReturning(ctx, db, i*batchRows)
			require.NoError(t, err)
			require.Equal(t, batchRows, n)
			require.NoError(t, db.Close())
		})
	}
}

// with a single server connection the release of a closed client session
// is the only thing that lets the next session through.
func TestDriverSequentialSessionsReleaseSingleConnection(t *testing.T) {
	socketPath, _ := startTestServer(t, duckproxyserver.WithMaxConnections(1), duckproxyserver.WithAcquireTimeout(2*time.Second))

	// the setup client is closed before the sessions begin so that it doesn't hold the only connection.
	setup, err := sql.Open("duckproxy", socketPath)
	require.NoError(t, err)
	_, err = setup.ExecContext(context.Background(), batchSchema)
	require.NoError(t, err)
	require.NoError(t, setup.Close())

	for i := range 4 {
		t.Run(fmt.Sprintf("session_%d", i), func(t *testing.T) {
			ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()

			db, err := sql.Open("duckproxy", socketPath)
			require.NoError(t, err)

			n, err := insertReturning(ctx, db, i*batchRows)
			require.NoError(t, err)
			require.Equal(t, batchRows, n)
			require.NoError(t, db.Close())
		})
	}
}
