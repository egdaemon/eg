// Package egworkloads scans a directory for eg workloads and records what it
// finds into the local analytics database, so that data is available
// wherever analytics.db is processed later.
package egworkloads

import (
	"context"
	_ "embed"
	"time"

	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
)

//go:embed migrations/0001_workloads.sql
var schema string

// Detect scans dir for eg workloads and records what it finds.
func Detect(ctx context.Context, q sqlx.Queryer, dir string) error {
	if _, err := q.ExecContext(ctx, schema); err != nil {
		return err
	}

	ts := time.Now()
	for d := range workspaces.Workloads(ctx, dir).Each(ctx) {
		w := Discovered{ID: uuid.Must(uuid.NewV7()).String(), Path: d.Path, Ts: ts}
		if err := DiscoveredInsert(ctx, q, w).Scan(&w); err != nil {
			return err
		}
	}

	return nil
}
