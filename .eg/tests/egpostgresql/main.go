package main

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
	"github.com/egdaemon/eg/runtime/x/wasi/egpostgresql"
)

//go:embed .psqlskel
var psqlskel embed.FS

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
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	log.Println("WAAAAAAAT")
	log.Println(egfs.Inspect(ctx, psqlskel))
	if err := CloneFS(ctx, egenv.WorkloadDirectory(), ".", psqlskel); err != nil {
		return err
	}

	runtime := egpostgresql.Runtime()
	return eg.Sequential(
		egpostgresql.Auto,
		egpostgresql.RecreateDatabase("test_database"),
		shell.Op(
			runtime.New("pwd"),
			runtime.New("pwd").Directory(egenv.WorkloadDirectory()),
			runtime.New("tree -L 1 .").Directory(egenv.WorkloadDirectory()),
			runtime.New("psql --no-psqlrc -d test_database --file .eg/tests/schema.sql"),
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

func CloneFS(ctx context.Context, dstdir string, rootdir string, archive fs.FS) (err error) {
	return fs.WalkDir(archive, rootdir, func(path string, d fs.DirEntry, err error) error {
		log.Println("DERP DERP 0", path)
		defer log.Println("DERP DERP 1", path)
		if err != nil {
			return err
		}

		// allow clone tree to be cancellable.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() && rootdir == path {
			info, err := d.Info()
			if err != nil {
				return err
			}

			return os.MkdirAll(dstdir, info.Mode().Perm())
		}

		rel := strings.TrimPrefix(path, rootdir)
		if rootdir == path {
			rel = path
		}

		dst := filepath.Join(dstdir, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		log.Println("cloning", rootdir, path, "->", dst, info.Mode().Perm())

		if d.IsDir() {
			return os.MkdirAll(dst, info.Mode().Perm())
		}

		if !d.IsDir() && rootdir == path {
			if err = os.MkdirAll(filepath.Dir(dst), 0777); err != nil {
				return err
			}
		}

		c, err := archive.Open(path)
		if err != nil {
			return err
		}
		defer c.Close()

		df, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		defer df.Close()

		if _, err := io.Copy(df, c); err != nil {
			return err
		}

		return nil
	})
}
