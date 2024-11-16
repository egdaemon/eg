package egpostgresql

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// configure the locally running instance of postgresql for use by the root user.
func Auto(ctx context.Context, _ eg.Op) (err error) {
	runtime := shell.Runtime()
	return shell.Run(
		ctx,
		runtime.New("pg_isready").Attempts(15), // 15 attempts = ~3seconds
		runtime.New("echo \"local all all trust\nhost all all 127.0.0.1/32 trust\" > \"$(su postgres -l -c \"psql --no-psqlrc -U postgres -d postgres -q -At -c 'SHOW hba_file;'\")\""),
		runtime.New("su postgres -l -c 'psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\"'"),
		runtime.New("su postgres -l -c 'psql --no-psqlrc -U postgres -d postgres -c \"CREATE ROLE root WITH SUPERUSER LOGIN\"'"),
	)
}
