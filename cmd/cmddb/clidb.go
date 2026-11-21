package cmddb

import (
	"context"
	"database/sql"
	"log"

	"github.com/egdaemon/eg/cmd/migrations"
)

// CLIDB is the cli's database with its migrations applied. it is injected into the commands that ask for it,
// and only opened for those. the database is optional for the commands using it, when it cannot be opened the
// failure is logged and DB is nil.
type CLIDB struct {
	*sql.DB
}

// NewCLIDB opens the cli's database, which applies its migrations. see migrations.OpenLocked.
func NewCLIDB(ctx context.Context) *CLIDB {
	db, err := migrations.OpenLocked(ctx, migrations.Path(migrations.CLI))
	if err != nil {
		log.Println("unable to open the cli database", err)
		return &CLIDB{}
	}

	return &CLIDB{DB: db}
}

// Close the database, if it was opened.
func (t CLIDB) Close() error {
	if t.DB == nil {
		return nil
	}

	return t.DB.Close()
}
