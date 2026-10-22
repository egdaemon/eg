// Package migrations the goose migrations for the cli's local duckdb database, shared by
// the lan peers and the remembered accounts.
package migrations

import (
	"context"
	"database/sql"
	"embed"

	"github.com/egdaemon/eg/internal/goosex"
	"github.com/egdaemon/eg/internal/userx"
)

// the database files. duckdb is single writer per file so a daemon and a cli sharing a
// cache directory must not share a file.
const (
	Runner = "runner.db"
	CLI    = "cli.db"
)

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
