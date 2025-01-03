// Package egpostgresql provide functionality for setting up
// a postgresql service within eg environments. Specifically
// allows for waiting for postgresql to be available,
// and configuring local access.
package egpostgresql

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// configure the locally running instance of postgresql for use by local users.
func Auto(ctx context.Context, _ eg.Op) (err error) {
	runtime := shell.Runtime().Privileged()
	return shell.Run(
		ctx,
		runtime.New("pg_isready").Attempts(15), // 15 attempts = ~3seconds
		runtime.New("echo \"local all all trust\nhost all all 127.0.0.1/32 trust\" > \"$(su postgres -l -c \"psql --no-psqlrc -U postgres -d postgres -q -At -c 'SHOW hba_file;'\")\""),
		runtime.New("su postgres -l -c 'psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\"'"),
		runtime.New("su postgres -l -c 'psql --no-psqlrc -U postgres -d postgres -c \"CREATE ROLE root WITH SUPERUSER LOGIN\"'"),
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
		runtime := shell.Runtime().Privileged()
		return shell.Run(
			ctx,
			runtime.Newf("psql --no-psqlrc -d postgres -c \"CREATE ROLE \"%s\" WITH SUPERUSER LOGIN\"", name),
		)
	}
}
