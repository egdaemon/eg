package ffiegcontainer

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero/api"
)

func Build(builder func(ctx context.Context, name, definition string) (*exec.Cmd, error)) func(
	ctx context.Context,
	m api.Module,
	nameoffset uint32, namelen uint32,
	definitionoffset uint32, definitionlen uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		nameoffset uint32, namelen uint32,
		definitionoffset uint32, definitionlen uint32,
	) uint32 {
		var (
			err        error
			name       string
			definition string
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

		if cmd, err = builder(ctx, name, definition); err != nil {
			log.Println("generating eg container failed", err)
			return 2
		}

		if err = cmd.Run(); err != nil {
			log.Println("generating eg container failed", err)
			return 3
		}

		return 0
	}
}

func Run(runner func(ctx context.Context, name, modulepath string) (err error)) func(
	ctx context.Context,
	m api.Module,
	nameoffset uint32, namelen uint32,
	definitionoffset uint32, definitionlen uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		nameoffset uint32, namelen uint32,
		modulepathoffset uint32, modulepathlen uint32,
	) uint32 {
		var (
			err        error
			name       string
			modulepath string
		)

		if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
			log.Println("unable to decode container name", err)
			return 1
		}

		if modulepath, err = ffi.ReadString(m.Memory(), modulepathoffset, modulepathlen); err != nil {
			log.Println("unable to decode modulepath", err)
			return 1
		}

		if err = runner(ctx, name, modulepath); err != nil {
			log.Println("generating eg container failed", err)
			return 2
		}

		return 0
	}
}

func PodmanBuild(ctx context.Context, name string, dir string, definition string) (cmd *exec.Cmd, err error) {
	return exec.CommandContext(ctx, "podman", "build", "--timestamp", "0", "-t", name, "-f", definition, dir), nil
}

func PodmanRun(ctx context.Context, cmdctx func(*exec.Cmd) *exec.Cmd, image, cname, rootdir, moduledir, modulepath, egbinpath string) (err error) {
	var (
		cmd *exec.Cmd
	)

	log.Println("running", image, cname)

	defer func() {
		cctx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()

		// don't care about this error; if the container doesn't exist its fine; if something
		// actually prevented it from stopped then our startup command will fail.
		if err = cmdctx(exec.CommandContext(cctx, "podman", "stop", cname)).Run(); err != nil {
			log.Println(err)
			return
		}

		// don't care about this error; if the container doesn't exist its fine; if something
		// actually prevented it from being rm then our startup command will fail.
		if err = cmdctx(exec.CommandContext(cctx, "podman", "rm", cname)).Run(); err != nil {
			log.Println(err)
			return
		}
	}()

	cmd = exec.CommandContext(
		ctx,
		"podman", "run",
		"--name", cname,
		"--detach",
		"--volume", fmt.Sprintf("%s:/opt/egmodule.wasm:ro", modulepath),
		"--volume", fmt.Sprintf("%s:/opt/egbin:ro", egbinpath),
		"--volume", fmt.Sprintf("%s:/opt/eg:O", rootdir),
		"--env", "CI",
		"--env", "EG_CI",
		"--env", "EG_RUNID",
		image,
		"/usr/sbin/init",
	)

	if err = cmdctx(cmd).Run(); err != nil {
		return err
	}

	cmd = exec.CommandContext(
		ctx,
		"podman", "exec", cname,
		"/opt/egbin",
		"module",
		"--directory=/opt/eg",
		"--moduledir", moduledir,
		"/opt/egmodule.wasm",
	)

	if err = cmdctx(cmd).Run(); err != nil {
		return err
	}

	return nil
}
