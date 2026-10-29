package runners

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg"
	_ "github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/gofrs/uuid/v5"
)

// SnapshotAnalytics writes a consistent copy of the analytics database of the running workload uid into dst.
// the database is held open by the workload's runner process, so the snapshot is taken by that process, through its
// duckproxy socket, using duckdb's ATTACH and COPY FROM DATABASE. returns an error matching fs.ErrNotExist
// when the workload isn't running.
func (t SpoolDirs) SnapshotAnalytics(ctx context.Context, uid uuid.UUID, dst io.Writer) (err error) {
	var (
		runtimedir = t.Path(uid).Resolve(workdirname, eg.RuntimeDirectory)
		socket     = filepath.Join(runtimedir, eg.SocketAnalytics)
		dbname     string
		dbpath     string
	)

	if _, err = os.Stat(socket); err != nil {
		return errorsx.Wrap(err, "workload analytics unavailable")
	}

	db, err := sql.Open("duckproxy", socket)
	if err != nil {
		return errorsx.Wrap(err, "unable to connect to analytics")
	}
	defer db.Close()

	// the runner process may see the runtime directory at a different path than we do,
	// so the snapshot is placed beside the database as the runner process sees it.
	if err = db.QueryRowContext(ctx, "SELECT database_name, path FROM duckdb_databases() WHERE database_name = current_database()").Scan(&dbname, &dbpath); err != nil {
		return errorsx.Wrap(err, "unable to locate analytics database")
	}

	snapname := "snapshot." + uuid.Must(uuid.NewV7()).String() + ".db"
	snappath := filepath.Join(filepath.Dir(dbpath), snapname)
	local := filepath.Join(runtimedir, snapname)
	defer func() { errorsx.Log(errorsx.Wrap(os.Remove(local), "unable to remove analytics snapshot")) }()

	const alias = "egsnapshot"
	quote := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
	ident := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	if _, err = db.ExecContext(ctx, "ATTACH '"+quote(snappath)+"' AS "+alias); err != nil {
		return errorsx.Wrap(err, "unable to attach analytics snapshot")
	}

	if _, err = db.ExecContext(ctx, "COPY FROM DATABASE "+ident(dbname)+" TO "+alias); err != nil {
		errorsx.Log(errorsx.Wrap(detach(ctx, db, alias), "unable to detach analytics snapshot"))
		return errorsx.Wrap(err, "unable to snapshot analytics")
	}

	// detaching checkpoints the snapshot so the file is complete.
	if err = detach(ctx, db, alias); err != nil {
		return errorsx.Wrap(err, "unable to detach analytics snapshot")
	}

	in, err := os.Open(local)
	if errors.Is(err, fs.ErrNotExist) {
		return errorsx.Wrap(err, "analytics snapshot unavailable")
	} else if err != nil {
		return errorsx.Wrap(err, "unable to open analytics snapshot")
	}
	defer in.Close()

	_, err = io.Copy(dst, in)
	return errorsx.Wrap(err, "unable to copy analytics snapshot")
}

func detach(ctx context.Context, db *sql.DB, alias string) error {
	_, err := db.ExecContext(ctx, "DETACH "+alias)
	return err
}
