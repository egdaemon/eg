package daemons

import (
	"database/sql"
	"log"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/egdaemon/eg/internal/userx"
)

// DatabaseProxy serves the daemon's database on its unix socket (see migrations.DatabaseSocketName) so other
// processes, e.g. eg compute lan --database daemon.db, can use it while the daemon has the file open: duckdb only allows a single
// process to have a database file open.
func DatabaseProxy(global *cmdopts.Global, db *sql.DB) {
	global.Cleanup.Add(1)
	go func() {
		defer global.Cleanup.Done()

		srv := duckproxyserver.New(db)
		if err := duckproxyserver.ListenUnix(global.Context, userx.DefaultRuntimeDirectory(migrations.DatabaseSocketName), srv); err != nil && global.Context.Err() == nil {
			log.Println("database proxy failed", err)
		}
	}()
}
