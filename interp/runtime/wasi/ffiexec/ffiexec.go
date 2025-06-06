package ffiexec

import (
	"context"
	"log"
	"os/exec"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero/api"
)

func Exec(op func(*exec.Cmd) *exec.Cmd) func(
	ctx context.Context,
	m api.Module,
	deadline int64,
	diroffset uint32, dirlen uint32,
	envoffset uint32, envlen uint32, envsize uint32,
	nameoffset uint32, namelen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64,
		diroffset uint32, dirlen uint32,
		envoffset uint32, envlen uint32, envsize uint32,
		nameoffset uint32, namelen uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		ictx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		cmd, err := Command(ictx, m, diroffset, dirlen, envoffset, envlen, envsize, nameoffset, namelen, argsoffset, argslen, argssize)
		if err != nil {
			log.Println("unable to create command", err)
			return 127
		}

		cmd = op(cmd)

		if err = execx.MaybeRun(cmd); err != nil {
			log.Println("failed to execute shell command", cmd.Dir, cmd.String(), err)
			return 128
		}

		return 0
	}
}

func Command(
	ctx context.Context,
	m api.Module,
	diroffset uint32, dirlen uint32,
	envoffset uint32, envlen uint32, envsize uint32,
	nameoffset uint32, namelen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) (_ *exec.Cmd, err error) {
	var (
		dir     string
		name    string
		args    []string
		environ []string
	)

	if dir, err = ffi.ReadString(m.Memory(), diroffset, dirlen); err != nil {
		return nil, errorsx.Wrap(err, "unable to read command name argument")
	}

	if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
		return nil, errorsx.Wrap(err, "unable to read command name argument")
	}

	if args, err = ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize); err != nil {
		return nil, errorsx.Wrap(err, "unable to read command arguments")
	}

	if environ, err = ffi.ReadStringArray(m.Memory(), envoffset, envlen, envsize); err != nil {
		return nil, errorsx.Wrap(err, "unable to read command environment")
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = environ

	return cmd, nil
}
