//go:build genieql.generate

package computesess

import (
	"context"

	"github.com/egdaemon/eg/internal/sqlx"
	genieql "github.com/james-lawrence/genieql/ginterp"
)

func Session(gql genieql.Structure) {
	gql.From(gql.Table("serve_sessions"))
}

func SessionScanner(gql genieql.Scanner, pattern func(s Session)) {}

// SessionUpsert records the session as running. started_at only moves when a stopped session is restarted,
// the port changing within a run, i.e. another module's container, is not a restart.
func SessionUpsert(
	gql genieql.Insert,
	pattern func(ctx context.Context, q sqlx.Queryer, s Session) NewSessionScannerStaticRow,
) {
	gql.Into("serve_sessions").Default("started_at", "stopped_at").Conflict(
		"ON CONFLICT (id) DO UPDATE SET module = EXCLUDED.module, directory = EXCLUDED.directory, pid = EXCLUDED.pid, ssh_requested = EXCLUDED.ssh_requested, ssh_port = EXCLUDED.ssh_port, ssh_user = EXCLUDED.ssh_user, ssh_key_path = EXCLUDED.ssh_key_path, started_at = CASE WHEN serve_sessions.stopped_at IS NULL THEN serve_sessions.started_at ELSE now() END, stopped_at = NULL",
	)
}

// SessionFindByID a session by id, running or stopped.
func SessionFindByID(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, id string) NewSessionScannerStaticRow,
) {
	gql = gql.Query(`SELECT ` + SessionScannerStaticColumns + ` FROM serve_sessions WHERE id = {id}`)
}

// SessionLookupAll the sessions, the running ones first and then the most recently started.
func SessionLookupAll(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer) NewSessionScannerStatic,
) {
	gql = gql.Query(`SELECT ` + SessionScannerStaticColumns + ` FROM serve_sessions ORDER BY stopped_at IS NOT NULL, started_at DESC`)
}

// SessionLookupRunning the sessions that have not been stopped.
func SessionLookupRunning(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer) NewSessionScannerStatic,
) {
	gql = gql.Query(`SELECT ` + SessionScannerStaticColumns + ` FROM serve_sessions WHERE stopped_at IS NULL`)
}

// SessionStop marks a running session as stopped, the port is no longer published. stopped sessions are kept as history.
func SessionStop(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, id string) NewSessionScannerStaticRow,
) {
	gql = gql.Query(`UPDATE serve_sessions SET stopped_at = now(), ssh_port = 0 WHERE id = {id} AND stopped_at IS NULL RETURNING ` + SessionScannerStaticColumns)
}

// SessionPrune forgets the stopped sessions beyond the limit, oldest first.
func SessionPrune(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, limit int64) NewSessionScannerStatic,
) {
	gql = gql.Query(`DELETE FROM serve_sessions WHERE stopped_at IS NOT NULL AND id NOT IN (SELECT id FROM serve_sessions WHERE stopped_at IS NOT NULL ORDER BY stopped_at DESC LIMIT {limit}) RETURNING ` + SessionScannerStaticColumns)
}
