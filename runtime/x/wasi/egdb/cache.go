package egdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

// Cache persists a copy of the analytics database of this workload into the cache directory, making it available
// to [Latest] in later runs of the workload, including runs that do not go through the control plane.
// it is intended to be the last operation of the workload: it is only reached when everything before it succeeded.
func Cache(ctx context.Context, _ eg.Op) error {
	db, err := Open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	return snapshot(ctx, db, cachepath())
}

// cachepath is where Cache keeps the copy of the database of this workload, see Latest.
func cachepath() string {
	return egenv.CacheDirectory(_eg.AnalyticsCachePath(egenv.WorkloadName()))
}

// snapshot writes a consistent copy of the database to path, replacing any earlier copy.
// the file is written by the process serving the database.
func snapshot(ctx context.Context, db *sql.DB, path string) (err error) {
	var name string
	if err = db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); err != nil {
		return errorsx.Wrap(err, "unable to determine the database")
	}

	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errorsx.Wrap(err, "unable to create the cache directory")
	}

	// copying into an existing file would collide with its tables.
	errorsx.Log(errorsx.Ignore(os.Remove(path), os.ErrNotExist))
	errorsx.Log(errorsx.Ignore(os.Remove(path+".wal"), os.ErrNotExist))

	if _, err = db.ExecContext(ctx, fmt.Sprintf("ATTACH '%s' AS snapshot", path)); err != nil {
		return errorsx.Wrap(err, "unable to create the cache file")
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf("COPY FROM DATABASE %s TO snapshot", name))
	// detaching checkpoints the copy into the file.
	if _, derr := db.ExecContext(ctx, "DETACH snapshot"); err == nil {
		err = derr
	}

	return errorsx.Wrap(err, "unable to copy the analytics database")
}
