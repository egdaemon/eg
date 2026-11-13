package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

const (
	// must match the environment variable set by `eg compute cache delete`.
	envpaths = "EG_CACHE_DELETE_PATHS"
	// the single path being removed by script.
	envpath = "EG_CACHE_DELETE_PATH"
)

// script removes $EG_CACHE_DELETE_PATH relative to the working directory, which is the cache root.
// the removal must never leave the cache:
//   - every symlink within the path's parent is resolved and the result must be strictly within the resolved cache root.
//   - the final component is never followed, a symlink is removed itself rather than its target.
//   - rm never crosses into another file system, i.e.) a mount within the cache.
//   - the path is passed through the environment, never interpolated into the script.
const script = `set -euo pipefail
root="$(realpath -e -- .)"
name="$(basename -- "${EG_CACHE_DELETE_PATH}")"
case "${name}" in
  .|..|/) echo "refusing to remove ${EG_CACHE_DELETE_PATH}: not a cache entry" >&2; exit 1 ;;
esac
parent="$(realpath -m -- "$(dirname -- "${EG_CACHE_DELETE_PATH}")")"
target="${parent}/${name}"
case "${target}" in
  "${root}"/?*) rm -rf --one-file-system -- "${target}" ;;
  *) echo "refusing to remove ${EG_CACHE_DELETE_PATH}: resolves outside of the cache ${target}" >&2; exit 1 ;;
esac
`

// removes the cache relative paths given by `eg compute cache delete`.
func remove(ctx context.Context, op eg.Op) error {
	privileged := shell.Runtime().Privileged().Directory(egenv.DirectoryCache()).Timeout(egenv.TTL())
	cmds := []shell.Command{}
	for _, p := range filepath.SplitList(os.Getenv(envpaths)) {
		// the cli already rejects these, guard regardless; script guards against symlinks.
		if p = filepath.Clean(p); !filepath.IsLocal(p) || p == "." {
			return fmt.Errorf("refusing to remove path outside of the cache: %s", p)
		}

		cmds = append(cmds, privileged.Environ(envpath, p).New(script))
	}

	cmds = append(cmds, privileged.Newf("du -shc %s", egenv.DirectoryCache()))

	return shell.Run(ctx, cmds...)
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		remove,
	)

	if err != nil {
		log.Fatalln(err)
	}
}
