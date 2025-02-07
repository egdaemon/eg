package events

import (
	context "context"
	"database/sql"
	"log"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
)

func InitializeDB(ctx context.Context, path string) (err error) {
	var (
		db *sql.DB
	)

	if db, err = sql.Open("duckdb", path); err != nil {
		return errorsx.Wrap(err, "unable to create analytics.db")
	}
	defer db.Close()

	if err = PrepareDB(ctx, db); err != nil {
		return errorsx.Wrap(err, "unable to prepare analytics.db")
	}

	return nil
}

func PrepareDB(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "INSTALL json"); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, "LOAD json"); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS 'eg.metrics.custom' (id UUID PRIMARY KEY, name TEXT NOT NULL, name_md5 uuid GENERATED ALWAYS AS (md5(name)), ts TIMESTAMP NOT NULL, metric JSON NOT NULL)"); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS 'eg.metrics.operation' (id UUID PRIMARY KEY, name TEXT NOT NULL, name_md5 uuid GENERATED ALWAYS AS (md5(name)), ts TIMESTAMP NOT NULL, module TEXT NOT NULL, op TEXT NOT NULL, milliseconds INTERVAL NOT NULL)"); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS 'eg.metrics.coverage' (id UUID PRIMARY KEY, path TEXT NOT NULL, path_md5 uuid GENERATED ALWAYS AS (md5(path)), statements FLOAT4 NOT NULL, branches FLOAT4 NOT NULL)"); err != nil {
		return err
	}

	return nil
}

func RecordMetric(ctx context.Context, db *sql.DB, msgs ...*Message) error {
	for _, m := range msgs {
		switch evt := m.Event.(type) {
		case *Message_Metric:
			mz := langx.Autoderef(evt.Metric)
			if err := db.QueryRowContext(ctx, "INSERT INTO 'eg.metrics.custom' (id, name, ts, metric) VALUES (?, ?, ?, ?)", m.Id, mz.Name, time.UnixMicro(m.Ts), mz.FieldsJSON).Err(); err != nil {
				return err
			}
		case *Message_Op:
			mz := langx.Autoderef(evt.Op)
			if err := db.QueryRowContext(ctx, "INSERT INTO 'eg.metrics.operation' (id, name, ts, module, op, milliseconds) VALUES (?, ?, ?, ?, ?, INTERVAL (?) MILLISECONDS)", m.Id, mz.Name, time.UnixMicro(m.Ts), mz.Module, mz.Op, mz.Milliseconds).Err(); err != nil {
				return err
			}
		case *Message_Coverage:
			mz := langx.Autoderef(evt.Coverage)
			if err := db.QueryRowContext(ctx, "INSERT INTO 'eg.metrics.coverage' (id, path, statements, branches) VALUES (?, ?, ?, ?)", m.Id, mz.Path, mz.Statements, mz.Branches).Err(); err != nil {
				return err
			}

		default:
			log.Printf("unknown message received %T\n", evt)
		}
	}
	return nil
}
