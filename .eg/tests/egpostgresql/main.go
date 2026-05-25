package main

import (
	"context"
	"log"
	"net/netip"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
	"github.com/egdaemon/eg/runtime/x/wasi/egpostgresql"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	c1 := eg.Container("eg.postgresql.test")

	err := eg.Perform(
		ctx,
		eg.Build(
			eg.DefaultModule(),
		),
		eg.Build(
			c1.BuildFromFile(".eg/tests/egpostgresql/Containerfile"),
		),
		eg.Module(
			ctx,
			c1,
			Postgres,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}

func Postgres(ctx context.Context, op eg.Op) (err error) {
	if err := egfs.CloneFS(ctx, egenv.WorkspaceDirectory(), ".", egpostgresql.TestArchive()); err != nil {
		return err
	}

	runtime := egpostgresql.Runtime()
	return eg.Sequential(
		egpostgresql.Auto,
		egpostgresql.RecreateDatabase("test_database"),
		shell.Op(
			runtime.New("psql --no-psqlrc -d test_database --file psqlskel/schema.sql").Directory(egenv.WorkspaceDirectory()),
		),
		egpostgresql.InsertSuperuser("migrations"),
		egpostgresql.Trust(egunsafe.HostPrefixes()...),
		egpostgresql.Trust(netip.PrefixFrom(netip.IPv4Unspecified(), 0), netip.PrefixFrom(netip.IPv6Unspecified(), 0)),
		egbug.DebugFailure(
			egpostgresql.Restart("systemctl restart postgresql.service"),
			shell.Op(
				shell.New("journalctl --since -1m -u postgresql.service"),
			),
		),
	)(ctx, op)
}
