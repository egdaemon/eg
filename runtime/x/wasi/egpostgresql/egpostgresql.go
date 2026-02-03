// Package egpostgresql provides functionality for setting up
// a postgresql service within eg environments. Specifically
// allows for waiting for the postgresql service to become
// available and configuring local access.
package egpostgresql

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// Wait for postgresql to become available and then configure the instance for use by root and egd users.
func Auto(ctx context.Context, _ eg.Op) (err error) {
	runtime := shell.Runtime().As("postgres").Timeout(5*time.Second).Environ("PAGER", "")

	return shell.Run(
		ctx,
		runtime.New("pg_isready").Attempts(15), // 15 attempts = ~3seconds
		runtime.New("echo \"local all all trust\nhost all all 127.0.0.1/32 trust\nhost all all ::1/128 trust\" | tee $(psql --no-password --no-psqlrc -q -At -c \"SHOW hba_file;\") >& /dev/null"),
		runtime.New("psql --no-password -q -At -c \"SELECT pg_reload_conf();\" >& /dev/null"),
		runtime.New("psql --no-password -c \"CREATE ROLE root WITH SUPERUSER LOGIN\""),
		runtime.New("psql --no-password -c \"CREATE ROLE egd WITH SUPERUSER LOGIN\""),
		runtime.New("psql -c \"ALTER SYSTEM SET listen_addresses = '*';\""),
	)
}

// Add additional cidrs to hba.conf
func Trust(v ...netip.Prefix) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) (err error) {
		runtime := shell.Runtime().As("postgres").Timeout(5*time.Second).Environ("PAGER", "")
		sentinel := egunsafe.UnroutablePrefix()

		var sb strings.Builder
		for _, prefix := range v {
			if prefix == sentinel {
				log.Println("ignoring unroutable host", prefix)
				continue
			}

			sb.WriteString(fmt.Sprintf("host all all %s trust\\n", prefix.String()))
		}

		if sb.Len() == 0 {
			return nil
		}

		cmd := fmt.Sprintf(
			"HBA=$(psql -qAt -c 'SHOW hba_file;') && printf \"%s\" >> \"$HBA\"",
			sb.String(),
		)

		return shell.Run(
			ctx,
			runtime.New("pg_isready").Attempts(15),
			runtime.New(cmd),
			runtime.New("psql -qAt -c 'SELECT pg_reload_conf();' > /dev/null"),
		)
	}
}

// command to restart postgresql
func Restart(cmd string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) (err error) {
		runtime := shell.Runtime().As("postgres").Timeout(5*time.Second).Environ("PAGER", "")
		return shell.Run(
			ctx,
			runtime.New(cmd).Privileged(),
			runtime.New("pg_isready").Attempts(15), // 15 attempts = ~3seconds
		)
	}
}

// Forcibly recreate a database.
func RecreateDatabase(name string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) (err error) {
		runtime := Runtime().As("postgres")
		return shell.Run(
			ctx,
			runtime.Newf("psql --no-password --no-psqlrc -c \"DROP DATABASE IF EXISTS \"%s\" WITH (FORCE)\"", name),
			runtime.Newf("psql --no-password --no-psqlrc -c \"CREATE DATABASE \"%s\"\"", name),
		)
	}
}

// Create a superuser with the provided name.
func InsertSuperuser(name string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) (err error) {
		runtime := shell.Runtime().As("postgres").Timeout(5 * time.Second)
		return shell.Run(
			ctx,
			runtime.Newf("psql --no-password --no-psqlrc -c \"CREATE ROLE \"%s\" WITH SUPERUSER LOGIN\"", name),
		)
	}
}

// build a environment that sets up postgresql the standard postgresql variables.
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
	runtime := shell.Runtime().As("postgres").Timeout(5 * time.Second)
	for i := begin; i < end; i++ {
		if err := shell.Run(ctx, runtime.Newf("psql --no-password --no-psqlrc -U postgres -d postgres -p %d -q -At -c 'SELECT 1;' > /dev/null 2>&1", i)); err == nil {
			return i
		}
	}

	return 5432
}
