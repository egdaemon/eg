package ffiexec

import (
	"context"
	"log"
	"os/exec"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffi"
	"github.com/pkg/errors"
	"github.com/tetratelabs/wazero/api"
)

func Exec(op func(*exec.Cmd) *exec.Cmd) func(
	ctx context.Context,
	m api.Module,
	nameoffset uint32, namelen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		nameoffset uint32, namelen uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		cmd, err := Command(ctx, m, nameoffset, namelen, argsoffset, argslen, argssize)
		if err != nil {
			log.Println("unable to build command", err)
			return 127
		}

		if err = op(cmd).Run(); err != nil {
			log.Println("failed to execute shell command", err)
			return 128
		}

		return 0
	}
}

func Command(
	ctx context.Context,
	m api.Module,
	nameoffset uint32, namelen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) (_ *exec.Cmd, err error) {
	var (
		name string
	)

	if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
		log.Println("unable to read name argument", err)
		return nil, errors.Wrap(err, "unable to read command name argument")
	}

	args := make([]string, 0, argslen)
	for offset, i := argsoffset, uint32(0); i < argslen*2; offset, i = offset+8, i+2 {
		data, err := ffi.ReadArrayElement(m.Memory(), argsoffset+i*4, (i+1)*4)
		if err != nil {
			return nil, errors.Wrap(err, "unable to read args elemen")
		}
		args = append(args, string(data))
	}

	return exec.CommandContext(ctx, name, args...), nil
}
