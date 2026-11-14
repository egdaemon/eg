// Package ffiegvm lets a guest run a module in a machine whose root is an egiso.
package ffiegvm

import (
	"context"
	"log"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero/api"
)

// Run runs the module at modulepath with runner, in a machine booted from the egiso uri names, the
// running machine's own when blank, with environ added to its environment.
func Run(runner func(ctx context.Context, uri string, modulepath string, environ ...string) error) func(
	ctx context.Context,
	m api.Module,
	deadline int64,
	urioffset uint32, urilen uint32,
	modulepathoffset uint32, modulepathlen uint32,
	environoffset uint32, environlen uint32, environsize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64,
		urioffset uint32, urilen uint32,
		modulepathoffset uint32, modulepathlen uint32,
		environoffset uint32, environlen uint32, environsize uint32,
	) uint32 {
		var (
			err        error
			uri        string
			modulepath string
			environ    []string
		)

		cctx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		if uri, err = ffi.ReadString(m.Memory(), urioffset, urilen); err != nil {
			log.Println("unable to decode egiso uri", err)
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

		if err = runner(cctx, uri, modulepath, environ...); err != nil {
			log.Println("running the module in a machine failed", err)
			return 2
		}

		return 0
	}
}
