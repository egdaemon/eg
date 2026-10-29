package egdb

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/egdaemon/eg/runtime/wasi/eg"
)

// Check logs whether the analytics database is available where it is run, i.e.) whether Cache can work there, and
// whether a copy of it is already cached. a debugging aid, it never fails: what it found is in the log.
func Check(ctx context.Context, _ eg.Op) error {
	path := cachepath()
	_, err := os.Stat(path)
	log.Println("egdb cache file", path, "exists:", err == nil)

	db, err := Open(ctx)
	if err != nil {
		log.Println("egdb unavailable:", err)
		return nil
	}
	defer db.Close()

	return probe(ctx, db)
}

// probe logs the name of the database db is connected to, or why it can't be queried.
func probe(ctx context.Context, db *sql.DB) error {
	var name string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); err != nil {
		log.Println("egdb connected but unusable:", err)
		return nil
	}

	log.Println("egdb available, database:", name)
	return nil
}
