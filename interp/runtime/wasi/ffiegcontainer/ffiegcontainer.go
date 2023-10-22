package ffiegcontainer

import (
	"context"
	"log"
	"os/exec"
	"time"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero/api"
)

func mayberun(c *exec.Cmd) error {
	if c == nil {
		return nil
	}

	log.Println("running", c.String())
	return c.Run()
}

func Pull(builder func(ctx context.Context, name string, options ...string) (*exec.Cmd, error)) func(
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
		var (
			err     error
			name    string
			options []string
			cmd     *exec.Cmd
		)

		if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
			log.Println("unable to decode container name", err)
			return 1
		}

		if options, err = ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize); err != nil {
			log.Println("unable to decode options", err)
			return 1
		}

		if cmd, err = builder(ctx, name, options...); err != nil {
			log.Println("generating container build command failed", err)
			return 2
		}

		if err = mayberun(cmd); err != nil {
			log.Println("generating container failed", err)
			return 3
		}

		return 0
	}
}

func Build(builder func(ctx context.Context, name, definition string, options ...string) (*exec.Cmd, error)) func(
	ctx context.Context,
	m api.Module,
	nameoffset uint32, namelen uint32,
	definitionoffset uint32, definitionlen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		nameoffset uint32, namelen uint32,
		definitionoffset uint32, definitionlen uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		var (
			err        error
			name       string
			definition string
			options    []string
			cmd        *exec.Cmd
		)

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

		if cmd, err = builder(ctx, name, definition, options...); err != nil {
			log.Println("generating container build command failed", err)
			return 2
		}

		if err = mayberun(cmd); err != nil {
			log.Println("generating container failed", err)
			return 3
		}

		return 0
	}
}

func Run(runner func(ctx context.Context, name, modulepath string, cmd []string, options ...string) (err error)) func(
	ctx context.Context,
	m api.Module,
	nameoffset uint32, namelen uint32,
	modulepathoffset uint32, modulepathlen uint32,
	cmdoffset uint32, cmdlen uint32, cmdsize uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
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

		if err = runner(ctx, name, modulepath, cmd, options...); err != nil {
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
	nameoffset uint32, namelen uint32,
	modulepathoffset uint32, modulepathlen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
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

		if err = runner(ctx, name, modulepath, options...); err != nil {
			log.Println("generating eg container failed", err)
			return 2
		}

		return 0
	}
}

func PodmanPull(ctx context.Context, name string, options ...string) (cmd *exec.Cmd, err error) {
	args := []string{
		"pull", name,
	}
	args = append(args, options...)

	return exec.CommandContext(ctx, "podman", args...), nil
}

func PodmanBuild(ctx context.Context, name string, dir string, definition string, options ...string) (cmd *exec.Cmd, err error) {
	args := []string{
		"build", "--timestamp", "0", "-t", name, "-f", definition,
	}
	args = append(args, options...)
	args = append(args, dir)

	return exec.CommandContext(ctx, "podman", args...), nil
}

func PodmanRun(ctx context.Context, cmdctx func(*exec.Cmd) *exec.Cmd, image, cname string, command []string, options ...string) (err error) {
	var (
		cmd *exec.Cmd
	)

	log.Println("running", image, cname)

	defer func() {
		cctx, done := context.WithTimeout(ctx, 10*time.Second)
		defer done()

		// don't care about this error; if the container doesn't exist its fine; if something
		// actually prevented it from stopped then our startup command will fail.
		if err = mayberun(cmdctx(exec.CommandContext(cctx, "podman", "stop", cname))); err != nil {
			log.Println(err)
			return
		}

		// don't care about this error; if the container doesn't exist its fine; if something
		// actually prevented it from being rm then our startup command will fail.
		if err = mayberun(cmdctx(exec.CommandContext(cctx, "podman", "rm", cname))); err != nil {
			log.Println(err)
			return
		}
	}()

	cmd = exec.CommandContext(
		ctx, "podman", "run", "-it", "--name", cname,
	)
	cmd.Args = append(cmd.Args, options...)
	cmd.Args = append(cmd.Args, image)
	cmd.Args = append(cmd.Args, command...)

	if err = mayberun(cmdctx(cmd)); err != nil {
		return err
	}

	return nil
}

func PodmanModule(ctx context.Context, cmdctx func(*exec.Cmd) *exec.Cmd, image, cname, moduledir string, options ...string) (err error) {
	var (
		cmd *exec.Cmd
	)

	log.Println("running", image, cname)

	defer func() {
		cctx, done := context.WithTimeout(ctx, 10*time.Second)
		defer done()

		// don't care about this error; if the container doesn't exist its fine; if something
		// actually prevented it from stopped then our startup command will fail.
		if err = mayberun(cmdctx(exec.CommandContext(cctx, "podman", "stop", cname))); err != nil {
			log.Println(err)
			return
		}

		// don't care about this error; if the container doesn't exist its fine; if something
		// actually prevented it from being rm then our startup command will fail.
		if err = mayberun(cmdctx(exec.CommandContext(cctx, "podman", "rm", cname))); err != nil {
			log.Println(err)
			return
		}
	}()

	options = append([]string{
		"run",
		"--name", cname,
		"--detach",
		"--env", "CI",
		"--env", "EG_CI",
		"--env", "EG_RUN_ID",
	},
		options...,
	)
	options = append(options,
		image,
		"/usr/sbin/init",
	)
	cmd = exec.CommandContext(
		ctx,
		"podman",
		options...,
	)

	if err = mayberun(cmdctx(cmd)); err != nil {
		return err
	}

	cmd = exec.CommandContext(
		ctx,
		"podman", "exec", "-it", cname,
		// "/usr/bin/eg",
		"/opt/egbin",
		"module",
		"--directory=/opt/eg",
		"--moduledir", moduledir,
		"/opt/egmodule.wasm",
	)

	if err = mayberun(cmdctx(cmd)); err != nil {
		return err
	}

	return nil
}
