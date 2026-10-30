// Package egdb provides access to the analytics database of the workload.
package egdb

import (
	"context"
	"database/sql"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/wasinet/wasinet"
)

// Open a connection pool to the workload database served by the runner. connections are made when they are first used,
// use OpenVerify to know the database is reachable. callers must Close the result.
func Open(ctx context.Context) *sql.DB {
	return sql.OpenDB(duckproxy.NewConnector(egunsafe.RuntimeDirectory(eg.SocketAnalytics), wasinet.DialContext))
}

// OpenVerify a connection pool to the workload database served by the runner.
// the connection is verified before returning; callers must Close the result.
func OpenVerify(ctx context.Context) (*sql.DB, error) {
	db := Open(ctx)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errorsx.Wrap(err, "unable to connect to the analytics database")
	}

	return db, nil
}

// OpenMigrate is Open after migrating the database to the schema this version of eg writes, so a guest does not depend
// on the runner having created the same schema, e.g.) a runner older than the guest. migrations already applied are
// skipped. disabled by setting EG_COMPUTE_GUEST_MIGRATIONS_ENABLED to false, which makes it Open.
// the connection is verified before returning; callers must Close the result.
func OpenMigrate(ctx context.Context) (*sql.DB, error) {
	db, err := OpenVerify(ctx)
	if err != nil {
		return nil, err
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	if !env.Boolean(true, eg.EnvComputeGuestMigrationsEnabled) {
		return nil
	}

	return errorsx.Wrap(events.PrepareDB(ctx, db), "unable to migrate the analytics database")
}
