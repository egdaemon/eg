// Package egcache provides utilities for managing and resetting caches related
// to the 'eg' runtime environment, including its internal caches and
// associated containerization resources managed by Podman.
//
// The primary function, [Reset], offers a comprehensive cleanup operation
// designed to clear various cached data, ensuring a clean state for
// workloads.
package egcache

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// reset all eg caches to empty. since it'll destroy the cache this
// operation should only be run at the end of a workload, and
// usually just by itself.
func Reset(ctx context.Context, op eg.Op) error {
	privileged := shell.Runtime().Privileged().Directory("/")
	return shell.Run(
		ctx,
		privileged.Newf("du -shc %s", egenv.CacheDirectory()),
		privileged.New("echo rm -rf -- * .[!.]* .??*").Directory(egenv.CacheDirectory()),
		privileged.New("rm -rf -- * .[!.]* .??*").Directory(egenv.CacheDirectory()),
		privileged.New("podman system prune -a -f"),
	)
}
