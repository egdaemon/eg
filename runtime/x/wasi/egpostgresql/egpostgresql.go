// Package egpostgresql provide functionality for setting up
// a postgresql service within eg environments. Specifically
// allows for waiting for postgresql to be available,
// and configuring local access.
package egpostgresql

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// configure the locally running instance of postgresql for use by local users.
func Auto(ctx context.Context, _ eg.Op) (err error) {
	runtime := shell.Runtime().Privileged().Timeout(5 * time.Second)
	return shell.Run(
		ctx,
		runtime.New("pg_isready").Attempts(15), // 15 attempts = ~3seconds
		runtime.New("echo \"local all all trust\nhost all all 127.0.0.1/32 trust\nhost all all ::1/128 trust\" > \"$(su postgres -l -c \"psql --no-psqlrc -U postgres -d postgres -q -At -c 'SHOW hba_file;'\")\""),
		runtime.New("su postgres -l -c 'psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\"'"),
		runtime.New("su postgres -l -c 'psql --no-psqlrc -U postgres -d postgres -c \"CREATE ROLE root WITH SUPERUSER LOGIN\"'"),
		runtime.Newf("su postgres -l -c 'psql --no-psqlrc -U postgres -d postgres -c \"CREATE ROLE egd WITH SUPERUSER LOGIN\"'"),
	)
}

// Forcibly creates and destroys a database. Should be run after Auto has initialized the default user.
func RecreateDatabase(name string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) (err error) {
		runtime := shell.Runtime().Privileged()
		return shell.Run(
			ctx,
			runtime.Newf("psql --no-psqlrc -d postgres -c \"DROP DATABASE IF EXISTS \"%s\" WITH (FORCE)\"", name),
			runtime.Newf("psql --no-psqlrc -d postgres -c \"CREATE DATABASE \"%s\"\"", name),
		)
	}
}

// Create a superuser with the provided name. Should be run after Auto has initialized the default user.
func InsertSuperuser(name string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) (err error) {
		runtime := shell.Runtime().Privileged().Timeout(5 * time.Second)
		return shell.Run(
			ctx,
			runtime.Newf("psql --no-psqlrc -d postgres -c \"CREATE ROLE \"%s\" WITH SUPERUSER LOGIN\"", name),
		)
	}
}

// attempt to build a environment that sets up
// the postgresql.
func Environ() []string {
	ctx, done := context.WithTimeout(context.Background(), 3*time.Second)
	defer done()
	return langx.Must(envx.Build().FromEnv(os.Environ()...).
		Var("PGPORT", fmt.Sprintf("%d", AutoLocatePort(ctx))).
		Var("PGHOST", "localhost").
		Environ())
}

// Create a shell runtime that properly
// sets up the postgresql environment.
func Runtime() shell.Command {
	return shell.Runtime().
		EnvironFrom(Environ()...)
}

// attempts to determine what port postgresql is listening on
func AutoLocatePort(ctx context.Context) int {
	return LocatePort(ctx, 5432, 5500)
}

// determine what port postgresql is listening on within a given range.
// if it can't determine the port it returns the default pg port 5432.
func LocatePort(ctx context.Context, begin, end int) int {
	for i := begin; i < end; i++ {
		if err := shell.Run(ctx, shell.Newf("psql --no-psqlrc -U postgres -d postgres -p %d -q -At -c 'SELECT 1;' > /dev/null 2>&1", i).Timeout(5*time.Second)); err == nil {
			return i
		}
	}

	return 5432
}
