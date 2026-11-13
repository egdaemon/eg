package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// must match the payload name used by `eg compute cache insert`.
const payload = "cache.payload.tar.gz"

// extracts the payload uploaded by `eg compute cache insert` into the cache directory.
func insert(ctx context.Context, op eg.Op) error {
	privileged := shell.Runtime().Privileged().Directory("/").Timeout(egenv.TTL())
	return shell.Run(
		ctx,
		privileged.Newf("tar -xzf %s -C %s", egunsafe.RuntimeDirectory(payload), egenv.DirectoryCache()),
		privileged.Newf("du -shc %s", egenv.DirectoryCache()),
	)
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		insert,
	)

	if err != nil {
		log.Fatalln(err)
	}
}
