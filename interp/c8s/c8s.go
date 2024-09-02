package c8s

import (
	context "context"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
)

func silence(c *exec.Cmd) *exec.Cmd {
	c.Stdout = io.Discard
	return c
}

func mayberun(c *exec.Cmd) error {
	if c == nil {
		return nil
	}

	debugx.Println("---------------", errorsx.Must(os.Getwd()), "running", c.Dir, "->", c.String(), "---------------")
	return c.Run()
}

func cleanup(ctx context.Context, cmdctx func(*exec.Cmd) *exec.Cmd, cname string) {
	cctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()

	// don't care about this error; if the container doesn't exist its fine; if something
	// actually prevented it from being stopped then our startup command will fail.
	if err := mayberun(silence(cmdctx(exec.CommandContext(cctx, "podman", "stop", cname)))); err != nil {
		log.Println(err)
		return
	}

	// don't care about this error; if the container doesn't exist its fine; if something
	// actually prevented it from being removed then our startup command will fail.
	if err := mayberun(silence(cmdctx(exec.CommandContext(cctx, "podman", "rm", cname)))); err != nil {
		log.Println(err)
		return
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
		"build", "--stdin", "--timestamp", "0", "-t", name, "-f", definition,
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

	defer cleanup(ctx, cmdctx, cname)

	cmd = exec.CommandContext(
		ctx, "podman", "run", "-i", "--name", cname,
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

	defer cleanup(ctx, cmdctx, cname)

	cmd = cmdctx(exec.CommandContext(
		ctx,
		"podman",
		PodmanModuleRunCmd(image, cname, moduledir, options...)...,
	))

	if err = mayberun(cmd); err != nil {
		return errorsx.Wrap(err, "unable to run container")
	}

	cmd = cmdctx(exec.CommandContext(
		ctx,
		"podman",
		PodmanModuleExecCmd(cname, moduledir)...,
	))

	if err = mayberun(cmd); err != nil {
		return errorsx.Wrap(err, "unable to exec module")
	}

	return nil
}

func PodmanModuleRunCmd(image, cname, moduledir string, options ...string) []string {
	options = append([]string{
		"run",
		"--name", cname,
		"--pids-limit", "-1",
		"--detach",
		"--env", "CI",
		"--env", "EG_CI",
		"--env", eg.EnvComputeRunID,
		"--env", eg.EnvComputeBin,
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
		envx.String("-i", eg.EnvComputeContainerExec),
		cname,
		envx.String("eg", eg.EnvComputeBin),
		"module",
		"--directory", "/opt/eg",
		"--moduledir", moduledir,
		"/opt/egmodule.wasm",
	}
}
