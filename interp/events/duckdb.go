package events

import (
	context "context"
	"database/sql"
	"log"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/internal/langx"
)

func PrepareMetrics(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "INSTALL json"); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, "LOAD json"); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS metrics (id UUID PRIMARY KEY, name TEXT NOT NULL, name_md5 uuid GENERATED ALWAYS AS (md5(name)), ts TIMESTAMP NOT NULL, metric JSON NOT NULL)"); err != nil {
		return err
	}

	return nil
}

func RecordMetric(ctx context.Context, db *sql.DB, msgs ...*Message) error {
	for _, m := range msgs {
		log.Println("DERP DERP", spew.Sdump(m))
		mz := langx.Autoderef(m.GetMetric())
		if err := db.QueryRowContext(ctx, "INSERT INTO metrics (id, name, ts, metric) VALUES (?, ?, ?, ?)", m.Id, mz.Name, time.UnixMicro(m.Ts), mz.FieldsJSON).Err(); err != nil {
			return err
		}
	}
	return nil
}
