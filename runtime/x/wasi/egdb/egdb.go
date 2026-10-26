// Package egdb provides access to the analytics database of the workload.
package egdb

import (
	"context"
	"database/sql"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/wasinet/wasinet"
)

// Open a connection pool to the workload database served by the runner.
// the connection is verified before returning; callers must Close the result.
func Open(ctx context.Context) (*sql.DB, error) {
	db := sql.OpenDB(duckproxy.NewConnector(egunsafe.RuntimeDirectory(eg.SocketAnalytics), wasinet.DialContext))

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errorsx.Wrap(err, "unable to connect to the analytics database")
	}

	return db, nil
}
