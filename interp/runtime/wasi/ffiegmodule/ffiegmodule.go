package ffiegmodule

import (
	"context"
	"log"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffi"
	"github.com/pkg/errors"
	"github.com/tetratelabs/wazero/api"
)

func Build(op func(refs ...string) error) func(
	ctx context.Context,
	m api.Module,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		var (
			err  error
			refs []string
		)
		if refs, err = genmodule(ctx, m, argsoffset, argslen, argssize); err != nil {
			log.Println("decoding arguments for eg module generation failed", err)
			return 1
		}

		if err = op(refs...); err != nil {
			log.Println("generating eg module failed", err)
			return 2
		}

		return 0
	}
}

func genmodule(
	ctx context.Context,
	m api.Module,
	argsoffset uint32, argslen uint32, argssize uint32,
) ([]string, error) {
	args := make([]string, 0, argslen)
	for offset, i := argsoffset, uint32(0); i < argslen*2; offset, i = offset+8, i+2 {
		data, err := ffi.ReadArrayElement(m.Memory(), argsoffset+i*4, (i+1)*4)
		if err != nil {
			return nil, errors.Wrap(err, "unable to read args elemen")
		}
		args = append(args, string(data))
	}

	return args, nil
}
