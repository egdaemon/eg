// Package migrations the goose migrations for the cli's local duckdb database, shared by
// the lan peers, the remembered accounts and the runner's compute history.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"os"
	"path/filepath"

	_ "github.com/duckdb/duckdb-go/v2" // registers the duckdb driver.
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/goosex"
	"github.com/egdaemon/eg/internal/userx"
)

// the database files. duckdb is single writer per file so a daemon and a cli sharing a
// cache directory must not share a file.
const (
	Daemon = "daemon.db"
	CLI    = "cli.db"
)

// DatabaseSocketName is the unix socket, within the runtime directory, the daemon serves its database on
// so other processes can use it while the daemon has the file open, see userx.DefaultRuntimeDirectory.
const DatabaseSocketName = "db.socket"

// FS the migrations, rooted at the sql files as goose expects.
//
//go:embed *.sql
var FS embed.FS

// Path is the location within the default cache directory, CACHE_DIRECTORY (systemd) first then
// XDG_CACHE_HOME, see userx.DefaultCacheDirectory.
func Path(ps ...string) string {
	return userx.DefaultCacheDirectory(ps...)
}

// Migrate applies the schema to an already opened database.
func Migrate(ctx context.Context, db *sql.DB) error {
	return goosex.Up(ctx, db, FS)
}

// Open the database at path, creating its directory and applying the migrations. the caller closes it.
// duckdb only allows a single process to have a database file open, share the returned handle
// within the process. use it for files with a single owner, i.e. the daemon's, see OpenLocked for the rest.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the database directory")
	}

	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to open the database")
	}

	return migrated(ctx, db)
}

// OpenLocked is Open for a database shared by concurrent commands: the command that connects first owns the file and
// serves it to the others, see sharedconnector. the database recovers if the owner closes its database or exits,
// the next connection takes the file over.
func OpenLocked(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the database directory")
	}

	return migrated(ctx, sql.OpenDB(newsharedconnector(path)))
}

func migrated(ctx context.Context, db *sql.DB) (*sql.DB, error) {
	if err := Migrate(ctx, db); err != nil {
		errorsx.Log(db.Close())
		return nil, err
	}

	return db, nil
}
