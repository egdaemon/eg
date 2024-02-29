package ffiegcontainer

import (
	"context"
	"log"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero/api"
)

func Pull(do func(ctx context.Context, name string, wdir string, options ...string) error) func(
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
		var (
			err     error
			wdir    string // TODO
			name    string
			options []string
		)

		cctx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
			log.Println("unable to decode container name", err)
			return 1
		}

		if options, err = ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize); err != nil {
			log.Println("unable to decode options", err)
			return 2
		}

		if err = do(cctx, name, wdir, options...); err != nil {
			log.Println("generating container failed", err)
			return 3
		}

		return 0
	}
}

func Build(do func(ctx context.Context, name, directory, definition string, options ...string) error) func(
	ctx context.Context,
	m api.Module,
	deadline int64,
	nameoffset uint32, namelen uint32,
	definitionoffset uint32, definitionlen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64,
		nameoffset uint32, namelen uint32,
		definitionoffset uint32, definitionlen uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		var (
			err        error
			name       string
			wdir       string = "." // TODO
			definition string
			options    []string
		)

		cctx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
			log.Println("unable to decode container name", err)
			return 1
		}

		if definition, err = ffi.ReadString(m.Memory(), definitionoffset, definitionlen); err != nil {
			log.Println("unable to decode container definition", err)
			return 1
		}

		if options, err = ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize); err != nil {
			log.Println("unable to decode options", err)
			return 1
		}

		if err = do(cctx, name, wdir, definition, options...); err != nil {
			log.Println("container build command failed", err)
			return 2
		}

		return 0
	}
}

func Run(runner func(ctx context.Context, name, modulepath string, cmd []string, options ...string) (err error)) func(
	ctx context.Context,
	m api.Module,
	deadline int64,
	nameoffset uint32, namelen uint32,
	modulepathoffset uint32, modulepathlen uint32,
	cmdoffset uint32, cmdlen uint32, cmdsize uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64,
		nameoffset uint32, namelen uint32,
		modulepathoffset uint32, modulepathlen uint32,
		cmdoffset uint32, cmdlen uint32, cmdsize uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		var (
			err        error
			name       string
			modulepath string
			cmd        []string
			options    []string
		)

		cctx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
			log.Println("unable to decode container name", err)
			return 1
		}

		if modulepath, err = ffi.ReadString(m.Memory(), modulepathoffset, modulepathlen); err != nil {
			log.Println("unable to decode modulepath", err)
			return 1
		}

		if cmd, err = ffi.ReadStringArray(m.Memory(), cmdoffset, cmdlen, cmdsize); err != nil {
			log.Println("unable to decode command", err)
			return 1
		}

		if options, err = ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize); err != nil {
			log.Println("unable to decode options", err)
			return 1
		}

		if err = runner(cctx, name, modulepath, cmd, options...); err != nil {
			log.Println("generating eg container failed", err)
			return 2
		}

		return 0
	}
}

// internal function for running modules
func Module(runner func(ctx context.Context, name, modulepath string, options ...string) (err error)) func(
	ctx context.Context,
	m api.Module,
	deadline int64,
	nameoffset uint32, namelen uint32,
	modulepathoffset uint32, modulepathlen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64,
		nameoffset uint32, namelen uint32,
		modulepathoffset uint32, modulepathlen uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		var (
			err        error
			name       string
			modulepath string
			options    []string
		)

		cctx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
			log.Println("unable to decode container name", err)
			return 1
		}

		if modulepath, err = ffi.ReadString(m.Memory(), modulepathoffset, modulepathlen); err != nil {
			log.Println("unable to decode modulepath", err)
			return 1
		}

		if options, err = ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize); err != nil {
			log.Println("unable to decode options", err)
			return 1
		}

		if err = runner(cctx, name, modulepath, options...); err != nil {
			log.Println("generating eg container failed", err)
			return 2
		}

		return 0
	}
}
