package c8s

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/errorhandling"
	"github.com/creack/pty"
	"github.com/docker/docker/api/types/container"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/langx"
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
		debugx.Println(errorsx.Wrap(err, "container stop failed"))
		return
	}

	// don't care about this error; if the container doesn't exist its fine; if something
	// actually prevented it from being removed then our startup command will fail.
	if err := execx.MaybeRun(silence(cmdctx(exec.CommandContext(cctx, "podman", "rm", cname)))); err != nil {
		debugx.Println(errorsx.Wrap(err, "container rm failed"))
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
	cmd := exec.CommandContext(
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
		PodmanModuleRunCmd(image, cname, options...)...,
	))

	if err = execx.MaybeRun(cmd); err != nil {
		return errorsx.Wrap(err, "unable to run container")
	}

	if err = moduleExec(ctx, cname, moduledir, cmd.Stdin, cmd.Stdout, cmd.Stderr); err != nil {
		return errorsx.Wrap(err, "unable to exec module")
	}

	return nil
}

func PodmanModuleRunCmd(image, cname string, options ...string) []string {
	args := make([]string, 0, len(options)+11)
	args = append(args,
		"run",
		"--name", cname,
		"--detach",
		"--replace",
		"--env", "CI",
		"--env", eg.EnvComputeRunID,
		"--env", eg.EnvComputeBin,
		"--env", eg.EnvComputeAccountID,
	)
	args = append(args, options...)
	args = append(args, image, "/usr/sbin/init")
	return args
}

// runcmd is md5 of the command that generated the container.
func moduleExec(ctx context.Context, cname, moduledir string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (err error) {
	var (
		result *define.InspectExecSession
	)

	// we don't close tty because ostensibly podman handles it.... this is probably
	// a bad assumption and a memory leak.
	ppty, tty, err := pty.Open()
	if err != nil {
		return err
	}

	// log.Println("moduleExec initiated", ppty.Name(), tty.Name())
	// defer log.Println("moduleExec completed", ppty.Name(), tty.Name())
	defer func() {
		errorsx.Log(
			errorsx.Wrap(
				errorsx.Compact(tty.Close(), ppty.Close()),
				"failed to close pty",
			),
		)
	}()

	rtty, wtty, err := os.Pipe()
	if err != nil {
		return errorsx.Wrap(err, "unable prepare pipe")
	}
	defer wtty.Close()

	id, err := containers.ExecCreate(ctx, cname, &handlers.ExecCreateConfig{
		ExecConfig: container.ExecOptions{
			Tty:          false,
			AttachStdin:  stdin != nil,
			AttachStderr: true,
			AttachStdout: true,
			Cmd: []string{
				envx.String("eg", eg.EnvComputeBin),
				"module",
				"--directory", eg.DefaultWorkingDirectory(),
				"--moduledir", moduledir,
				eg.DefaultMountRoot(eg.ModuleBin),
			},
		},
	})
	if err != nil {
		return errorsx.Wrap(err, "unable prepare exec session")
	}
	defer func() {
		// only attempt a force removal if we encountered an error
		if err == nil {
			return
		}

		errorsx.Log(errorsx.Wrap(containers.ExecRemove(ctx, id, &containers.ExecRemoveOptions{Force: langx.Autoptr(true)}), "failed to remove exec session"))
	}()

	go func() {
		debugx.Println("stdout copy initiated", ppty.Name())
		defer debugx.Println("stdout copy completed", ppty.Name())
		_, _ = io.Copy(stdout, ppty) // not important
	}()

	if stdin != nil {
		go func() {
			debugx.Println("stdin copy initiated", tty.Name())
			defer debugx.Println("stdin copy completed", tty.Name())
			_, _ = io.Copy(wtty, stdin) // not important
		}()
	}

	err = containers.ExecStartAndAttach(ctx, id, &containers.ExecStartAndAttachOptions{
		InputStream:  bufio.NewReader(rtty),
		OutputStream: langx.Autoptr(io.Writer(tty)),
		ErrorStream:  langx.Autoptr(io.Writer(stderr)),
		AttachOutput: langx.Autoptr(true),
		AttachError:  langx.Autoptr(true),
		AttachInput:  langx.Autoptr(true),
	})
	if err != nil {
		return errorsx.Wrap(err, "podman exec start failed")
	}

	if result, err = containers.ExecInspect(ctx, id, nil); err != nil {
		return err
	} else if result.ExitCode > 0 {
		return errorsx.Errorf("module failed with exit code: %d", result.ExitCode)
	}

	// soft removal.
	errorsx.Log(errorsx.Wrap(containers.ExecRemove(ctx, id, &containers.ExecRemoveOptions{}), "failed to remove exec session"))

	// wait for the exec session to disappear
	for {
		if _, err = containers.ExecInspect(ctx, id, nil); err != nil {
			if errm, ok := err.(*errorhandling.ErrorModel); ok && errm.Code() == 404 {
				return nil
			}
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
