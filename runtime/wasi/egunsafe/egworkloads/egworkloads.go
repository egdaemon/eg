// Package egworkloads scans a directory for eg workloads and records what it
// finds into the local analytics database, so that data is available
// wherever analytics.db is processed later.
package egworkloads

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/goosex"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
)

//go:embed migrations
var migrationsdir embed.FS

// migrations rooted at the directory's contents, matching os.DirFS semantics
// (goose expects migration filenames directly at the fs root).
var migrations = errorsx.Must(fs.Sub(migrationsdir, "migrations"))

// Detect scans dir for eg workloads and records what it finds.
func Detect(ctx context.Context, db *sql.DB, dir string) error {
	if err := goosex.Up(ctx, db, migrations); err != nil {
		return err
	}

	ts := time.Now()
	for d := range workspaces.Workloads(ctx, dir).Each(ctx) {
		w := Discovered{ID: uuid.Must(uuid.NewV7()).String(), Path: d.Path, Ts: ts}
		if err := DiscoveredInsert(ctx, db, w).Scan(&w); err != nil {
			return err
		}
	}

	return nil
}
