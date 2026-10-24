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
// within the process.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the database directory")
	}

	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to open the database")
	}

	if err = Migrate(ctx, db); err != nil {
		errorsx.Log(db.Close())
		return nil, err
	}

	return db, nil
}
