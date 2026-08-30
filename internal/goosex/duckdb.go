// Package goosex adapts goose (github.com/pressly/goose/v3) migrations for
// duckdb. It mirrors github.com/james-lawrence/genieql/internal/goosex, which
// genieql uses to apply these same migrations while generating query code, so
// running migrations here at runtime stays consistent with codegen time.
package goosex

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

// DuckdbStore implements goose's database.Store for duckdb, which speaks
// $1-style placeholders and needs an explicit sequence for the version table's
// auto-incrementing id.
type DuckdbStore struct{}

func (t DuckdbStore) Tablename() string {
	return goose.DefaultTablename
}

func (t DuckdbStore) CreateVersionTable(ctx context.Context, db database.DBTxConn) error {
	q := fmt.Sprintf(`CREATE SEQUENCE %s_seq;
		CREATE TABLE %s (
		id integer PRIMARY KEY DEFAULT NEXTVAL('%s_seq'),
		version_id bigint NOT NULL,
		is_applied boolean NOT NULL,
		tstamp timestamp NOT NULL DEFAULT now()
	)`, t.Tablename(), t.Tablename(), t.Tablename())
	_, err := db.ExecContext(ctx, q)
	return errorsx.Wrap(err, "unable to create versioned table")
}

func (t DuckdbStore) Insert(ctx context.Context, db database.DBTxConn, req database.InsertRequest) error {
	q := fmt.Sprintf(`INSERT INTO %s (version_id, is_applied) VALUES ($1, $2)`, t.Tablename())
	row := db.QueryRowContext(ctx, q, req.Version, true)
	return errorsx.Wrap(row.Err(), "unable to insert version")
}

func (t DuckdbStore) Delete(ctx context.Context, db database.DBTxConn, version int64) error {
	q := fmt.Sprintf(`DELETE FROM %s WHERE version_id=$1`, t.Tablename())
	row := db.QueryRowContext(ctx, q, version)
	return errorsx.Wrap(row.Err(), "unable to delete version")
}

func (t DuckdbStore) GetMigration(ctx context.Context, db database.DBTxConn, version int64) (*database.GetMigrationResult, error) {
	q := fmt.Sprintf(`SELECT tstamp, is_applied FROM %s WHERE version_id=$1 ORDER BY tstamp DESC LIMIT 1`, t.Tablename())
	var timestamp time.Time
	var isApplied bool
	err := db.QueryRowContext(ctx, q, version).Scan(&timestamp, &isApplied)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to load migration")
	}

	return &database.GetMigrationResult{
		IsApplied: isApplied,
		Timestamp: timestamp,
	}, nil
}

func (t DuckdbStore) GetLatestVersion(ctx context.Context, db database.DBTxConn) (id int64, err error) {
	q := fmt.Sprintf(`SELECT version_id from %s ORDER BY id DESC LIMIT 1`, t.Tablename())
	err = db.QueryRowContext(ctx, q).Scan(&id)
	return id, errorsx.Wrap(err, "unable to load latest migration")
}

func (t DuckdbStore) ListMigrations(ctx context.Context, db database.DBTxConn) ([]*database.ListMigrationsResult, error) {
	q := fmt.Sprintf(`SELECT version_id, is_applied from %s ORDER BY id DESC`, t.Tablename())
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var migrations []*database.ListMigrationsResult
	for rows.Next() {
		var version int64
		var isApplied bool
		if err := rows.Scan(&version, &isApplied); err != nil {
			return nil, err
		}
		migrations = append(migrations, &database.ListMigrationsResult{
			Version:   version,
			IsApplied: isApplied,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return migrations, nil
}

// Up applies every pending migration in fsys (a directory of goose-formatted
// .sql files) to db.
func Up(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	provider, err := goose.NewProvider("", db, fsys, goose.WithStore(DuckdbStore{}))
	if err != nil {
		return errorsx.Wrap(err, "unable to build migration provider")
	}

	if _, err := provider.Up(ctx); err != nil {
		return errorsx.Wrap(err, "unable to run migrations")
	}

	return nil
}
