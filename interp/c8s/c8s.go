package c8s

import (
	context "context"
	"io"
	"log"
	"os/exec"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
)

func silence(c *exec.Cmd) *exec.Cmd {
	c.Stdout = io.Discard
	return c
}

func cleanup(ctx context.Context, cmdctx func(*exec.Cmd) *exec.Cmd, cname string) {
	cctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()

	// don't care about this error; if the container doesn't exist its fine; if something
	// actually prevented it from being stopped then our startup command will fail.
	if err := execx.MaybeRun(silence(cmdctx(exec.CommandContext(cctx, "podman", "stop", cname)))); err != nil {
		log.Println(err)
		return
	}

	// don't care about this error; if the container doesn't exist its fine; if something
	// actually prevented it from being removed then our startup command will fail.
	if err := execx.MaybeRun(silence(cmdctx(exec.CommandContext(cctx, "podman", "rm", cname)))); err != nil {
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
		"build", "--stdin", "-t", name, "-f", definition,
	}
	args = append(args, options...)
	args = append(args, dir)

	return exec.CommandContext(ctx, "podman", args...), nil
}

func PodmanRun(ctx context.Context, cmdctx func(*exec.Cmd) *exec.Cmd, image, cname string, command []string, options ...string) (err error) {
	var (
		cmd *exec.Cmd
	)

	defer cleanup(ctx, cmdctx, cname)

	cmd = exec.CommandContext(
		ctx, "podman", "run", "-i", "--name", cname,
	)
	cmd.Args = append(cmd.Args, options...)
	cmd.Args = append(cmd.Args, image)
	cmd.Args = append(cmd.Args, command...)

	if err = execx.MaybeRun(cmdctx(cmd)); err != nil {
		return err
	}

	return nil
}

func PodmanPrune(ctx context.Context, cmdctx func(*exec.Cmd) *exec.Cmd) (err error) {
	var (
		cmd *exec.Cmd
	)

	cmd = exec.CommandContext(
		ctx, "podman", "system", "prune", "-f",
	)

	if err = execx.MaybeRun(cmdctx(cmd)); err != nil {
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

	if err = execx.MaybeRun(cmd); err != nil {
		return errorsx.Wrap(err, "unable to run container")
	}

	cmd = cmdctx(exec.CommandContext(
		ctx,
		"podman",
		PodmanModuleExecCmd(cname, moduledir)...,
	))

	if err = execx.MaybeRun(cmd); err != nil {
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
		"--env", eg.EnvComputeAccountID,
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
		"--directory", eg.DefaultRootDirectory,
		"--moduledir", moduledir,
		"/opt/egmodule.wasm",
	}
}
