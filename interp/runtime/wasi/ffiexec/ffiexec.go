package ffiexec

import (
	"context"
	"log"
	"os/exec"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffi"
	"github.com/pkg/errors"
	"github.com/tetratelabs/wazero/api"
)

func mayberun(c *exec.Cmd) error {
	if c == nil {
		return nil
	}

	return c.Run()
}

func Exec(op func(*exec.Cmd) *exec.Cmd) func(
	ctx context.Context,
	m api.Module,
	deadline int64,
	nameoffset uint32, namelen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64,
		nameoffset uint32, namelen uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		ictx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		cmd, err := Command(ictx, m, nameoffset, namelen, argsoffset, argslen, argssize)
		if err != nil {
			log.Println("unable to build command", err)
			return 127
		}

		log.Println("initiated", cmd.String())
		defer log.Println("completed", cmd.String())

		if err = mayberun(op(cmd)); err != nil {
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
		args []string
	)

	if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
		return nil, errors.Wrap(err, "unable to read command name argument")
	}

	if args, err = ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize); err != nil {
		return nil, errors.Wrap(err, "unable to read command arguments")
	}

	return exec.CommandContext(ctx, name, args...), nil
}
