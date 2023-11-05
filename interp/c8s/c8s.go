package c8s

import (
	context "context"
	"log"
	"os/exec"
	"time"

	"github.com/james-lawrence/eg/internal/envx"
)

func mayberun(c *exec.Cmd) error {
	if c == nil {
		return nil
	}

	log.Println("running", c.String())
	return c.Run()
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

	defer func() {
		cctx, done := context.WithTimeout(ctx, 10*time.Second)
		defer done()

		// don't care about this error; if the container doesn't exist its fine; if something
		// actually prevented it from being stopped then our startup command will fail.
		if err = mayberun(cmdctx(exec.CommandContext(cctx, "podman", "stop", cname))); err != nil {
			log.Println(err)
			return
		}

		// don't care about this error; if the container doesn't exist its fine; if something
		// actually prevented it from being removed then our startup command will fail.
		if err = mayberun(cmdctx(exec.CommandContext(cctx, "podman", "rm", cname))); err != nil {
			log.Println(err)
			return
		}
	}()

	cmd = exec.CommandContext(
		ctx,
		"podman",
		PodmanModuleRunCmd(image, cname, moduledir, options...)...,
	)

	if err = mayberun(cmdctx(cmd)); err != nil {
		return err
	}

	cmd = exec.CommandContext(
		ctx,
		"podman",
		PodmanModuleExecCmd(cname, moduledir)...,
	)

	if err = mayberun(cmdctx(cmd)); err != nil {
		return err
	}

	return nil
}

func PodmanModuleRunCmd(image, cname, moduledir string, options ...string) []string {
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

	return append(
		options,
		image,
		"/usr/sbin/init",
	)
}

func PodmanModuleExecCmd(cname, moduledir string) []string {
	return []string{
		"exec",
		"-it",
		cname,
		envx.String("eg", "EG_BIN"),
		"module",
		"--directory=/opt/eg",
		"--moduledir", moduledir,
		"/opt/egmodule.wasm",
	}
}
