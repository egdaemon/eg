package events

import (
	context "context"
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"time"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/goosex"
	"github.com/egdaemon/eg/internal/langx"
)

//go:embed migrations
var migrationsdir embed.FS

// migrations rooted at the directory's contents, goose expects migration filenames directly at the fs root.
var migrations = errorsx.Must(fs.Sub(migrationsdir, "migrations"))

func InitializeDB(ctx context.Context, path string) (err error) {
	debugx.Println("initialize analytics database initiated")
	defer debugx.Println("initialize analytics database completed")

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
	debugx.Println("prepare analytics database initiated")
	defer debugx.Println("prepare analytics database completed")

	dctx, done := context.WithTimeout(ctx, 15*time.Second)
	defer done()

	if _, err := db.ExecContext(dctx, "LOAD json"); err != nil {
		return err
	}

	return goosex.Up(dctx, db, migrations)
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
