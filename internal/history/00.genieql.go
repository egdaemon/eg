//go:build genieql.generate

package history

import (
	"context"

	"github.com/egdaemon/eg/internal/sqlx"
	genieql "github.com/james-lawrence/genieql/ginterp"
)

func Entry(gql genieql.Structure) {
	gql.From(gql.Table("compute_history"))
}

func EntryScanner(gql genieql.Scanner, pattern func(e Entry)) {}

// EntryUpsert records the outcome of a workload, replacing the earlier outcome of the same workload.
func EntryUpsert(
	gql genieql.Insert,
	pattern func(ctx context.Context, q sqlx.Queryer, e Entry) NewEntryScannerStaticRow,
) {
	gql.Into("compute_history").Conflict(
		"ON CONFLICT (id) DO UPDATE SET description = EXCLUDED.description, vcs_uri = EXCLUDED.vcs_uri, vcs_commit = EXCLUDED.vcs_commit, uploaded_by = EXCLUDED.uploaded_by, result = EXCLUDED.result, completed_at = EXCLUDED.completed_at, duration = EXCLUDED.duration, logs_path = EXCLUDED.logs_path, expires_at = EXCLUDED.expires_at",
	)
}

// EntryFindByID finds the outcome of a workload, unless it has expired.
func EntryFindByID(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, id string) NewEntryScannerStaticRow,
) {
	gql = gql.Query(`SELECT ` + EntryScannerStaticColumns + ` FROM compute_history WHERE id = {id} AND expires_at >= now()`)
}

// EntryPurgeExpired deletes and returns the outcomes that have expired.
func EntryPurgeExpired(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer) NewEntryScannerStatic,
) {
	gql = gql.Query(`DELETE FROM compute_history WHERE expires_at < now() RETURNING ` + EntryScannerStaticColumns)
}
