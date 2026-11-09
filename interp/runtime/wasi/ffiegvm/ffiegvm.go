// Package ffiegvm lets a guest run a module in a virtual machine whose root is an egiso.
package ffiegvm

import (
	"context"
	"log"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero/api"
)

// Run runs the module at modulepath with runner, in a machine whose root is the egiso at location pinned to toc,
// the digest of its table of contents, with environ added to its environment.
func Run(runner func(ctx context.Context, location string, toc string, modulepath string, environ ...string) error) func(
	ctx context.Context,
	m api.Module,
	deadline int64,
	locationoffset uint32, locationlen uint32,
	tocoffset uint32, toclen uint32,
	modulepathoffset uint32, modulepathlen uint32,
	environoffset uint32, environlen uint32, environsize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64,
		locationoffset uint32, locationlen uint32,
		tocoffset uint32, toclen uint32,
		modulepathoffset uint32, modulepathlen uint32,
		environoffset uint32, environlen uint32, environsize uint32,
	) uint32 {
		var (
			err        error
			location   string
			toc        string
			modulepath string
			environ    []string
		)

		cctx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		if location, err = ffi.ReadString(m.Memory(), locationoffset, locationlen); err != nil {
			log.Println("unable to decode egiso location", err)
			return 1
		}

		if toc, err = ffi.ReadString(m.Memory(), tocoffset, toclen); err != nil {
			log.Println("unable to decode egiso table of contents digest", err)
			return 1
		}

		if modulepath, err = ffi.ReadString(m.Memory(), modulepathoffset, modulepathlen); err != nil {
			log.Println("unable to decode modulepath", err)
			return 1
		}

		if environ, err = ffi.ReadStringArray(m.Memory(), environoffset, environlen, environsize); err != nil {
			log.Println("unable to decode environment", err)
			return 1
		}

		if err = runner(cctx, location, toc, modulepath, environ...); err != nil {
			log.Println("running the module in the vm failed", err)
			return 2
		}

		return 0
	}
}
