package c8sproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"github.com/containers/common/pkg/detach"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/errorhandling"
	"github.com/docker/docker/api/types/container"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/langx"

	xterm "golang.org/x/term"
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

	// cleanup has its own timeout we want to ensure the commands at least
	// attempt to cleanup.
	defer cleanup(context.Background(), cmdctx, cname)

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

	// cleanup has its own timeout we want to ensure the commands at least
	// attempt to cleanup.
	defer cleanup(context.Background(), cmdctx, cname)

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
		"--env", eg.EnvComputeBin,
	)
	args = append(args, options...)
	args = append(args, image, "/usr/sbin/init")
	return args
}

// runcmd is md5 of the command that generated the container.
func moduleExec(ctx context.Context, cname, moduledir string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (err error) {
	var (
		rtty, wtty *os.File
	)

	id, err := containers.ExecCreate(ctx, cname, &handlers.ExecCreateConfig{
		ExecOptions: container.ExecOptions{
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

	if stdin != nil {
		rtty, wtty, err = os.Pipe()
		if err != nil {
			return errorsx.Wrap(err, "unable prepare pipe")
		}
		defer rtty.Close()
		defer wtty.Close()

		go func() {
			_, _ = io.Copy(wtty, stdin) // not important
		}()
	}

	if err = execAttach(ctx, id, rtty, stdout, stderr); err != nil {
		return errorsx.Wrap(err, "podman exec attach failed")
	}

	// wait for the exec session to disappear
	for {
		if result, cause := containers.ExecInspect(ctx, id, nil); cause != nil {
			if errm, ok := cause.(*errorhandling.ErrorModel); ok && errm.Code() == 404 {
				return nil
			} else {
				return errorsx.Wrapf(cause, "unknown exec session error: %d %s", errm.Code(), errm.Error())
			}
		} else if result.ExitCode > 0 {
			return errorsx.Errorf("module failed with exit code: %d", result.ExitCode)
		} else if !result.Running {
			// soft removal.
			errorsx.Log(errorsx.Wrap(containers.ExecRemove(ctx, id, &containers.ExecRemoveOptions{}), "failed to remove exec session"))
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func execAttach(ctx context.Context, sessionID string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	isSet := struct {
		stdin  bool
		stdout bool
		stderr bool
	}{
		stdin:  !(stdin == nil || reflect.ValueOf(stdin).IsNil()),
		stdout: !(stdout == nil || reflect.ValueOf(stdout).IsNil()),
		stderr: !(stderr == nil || reflect.ValueOf(stderr).IsNil()),
	}
	// Ensure golang can determine that interfaces are "really" nil
	if !isSet.stdin {
		stdin = (io.Reader)(nil)
	}
	if !isSet.stdout {
		stdout = (io.Writer)(nil)
	}
	if !isSet.stderr {
		stderr = (io.Writer)(nil)
	}

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	// Unless all requirements are met, don't use "stdin" is a terminal
	file, ok := stdin.(*os.File)
	_, outOk := stdout.(*os.File)
	needTTY := ok && outOk && xterm.IsTerminal(int(file.Fd()))
	if needTTY {
		state, err := xterm.MakeRaw(int(file.Fd()))
		if err != nil {
			return err
		}
		defer func() {
			if err := xterm.Restore(int(file.Fd()), state); err != nil {
				log.Println("unable to restore terminal state", err)
			}
		}()
	}

	body := struct {
		Detach bool   `json:"Detach"`
		TTY    bool   `json:"Tty"`
		Height uint16 `json:"h"`
		Width  uint16 `json:"w"`
	}{
		Detach: false,
		TTY:    needTTY,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	var socket net.Conn
	socketSet := false
	dialContext := conn.Client.Transport.(*http.Transport).DialContext
	t := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			c, err := dialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			if !socketSet {
				socket = c
				socketSet = true
			}
			return c, err
		},
		IdleConnTimeout: time.Duration(0),
	}
	conn.Client.Transport = t
	// We need to inspect the exec session first to determine whether to use
	// -t.
	resp, err := conn.DoRequest(ctx, bytes.NewReader(bodyJSON), http.MethodPost, "/exec/%s/start", nil, nil, sessionID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !(resp.IsSuccess() || resp.IsInformational()) {
		defer resp.Body.Close()
		return resp.Process(nil)
	}

	if needTTY {
		winChange := make(chan os.Signal, 1)
		winCtx, winCancel := context.WithCancel(ctx)
		defer winCancel()
		signal.Notify(winChange, syscall.SIGWINCH)
		attachHandleResize(ctx, winCtx, winChange, sessionID, file)
	}

	stdoutChan := make(chan error)
	stdinChan := make(chan error, 1) // stdin channel should not block

	if isSet.stdin {
		go func() {
			_, err := detach.Copy(socket, stdin, []byte{})
			if err != nil && err != define.ErrDetach {
				log.Println("failed to write input to service:", err)
			}
			if err == nil {
				if closeWrite, ok := socket.(containers.CloseWriter); ok {
					if err := closeWrite.CloseWrite(); err != nil {
						debugx.Printf("Failed to close STDIN for writing: %v", err)
					}
				}
			}
			stdinChan <- err
		}()
	}

	buffer := make([]byte, 1024)
	if needTTY {
		go func() {
			// If not multiplex'ed, read from server and write to stdout
			_, err := io.Copy(stdout, socket)
			stdoutChan <- err
		}()

		for {
			select {
			case err := <-stdoutChan:
				if err != nil {
					return err
				}

				return nil
			case err := <-stdinChan:
				if err != nil {
					return err
				}

				return nil
			}
		}
	} else {
		for {
			// Read multiplexed channels and write to appropriate stream
			fd, l, err := containers.DemuxHeader(socket, buffer)
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					return nil
				}
				return err
			}
			frame, err := containers.DemuxFrame(socket, buffer, l)
			if err != nil {
				return err
			}

			switch {
			case fd == 0:
				if isSet.stdout {
					if _, err := stdout.Write(frame[0:l]); err != nil {
						return err
					}
				}
			case fd == 1:
				if isSet.stdout {
					if _, err := stdout.Write(frame[0:l]); err != nil {
						return err
					}
				}
			case fd == 2:
				if isSet.stderr {
					if _, err := stderr.Write(frame[0:l]); err != nil {
						return err
					}
				}
			case fd == 3:
				return fmt.Errorf("from service from stream: %s", frame)
			default:
				return fmt.Errorf("unrecognized channel '%d' in header, 0-3 supported", fd)
			}
		}
	}
}

// This is intended to not be run as a goroutine, handling resizing for a container
// or exec session. It will call resize once and then starts a goroutine which calls resize on winChange
func attachHandleResize(ctx, winCtx context.Context, winChange chan os.Signal, id string, file *os.File) {
	resize := func() {
		w, h, err := xterm.GetSize(int(file.Fd()))
		if err != nil {
			debugx.Println("Failed to obtain TTY size:", err)
		}

		err = containers.ResizeExecTTY(ctx, id, new(containers.ResizeExecTTYOptions).WithHeight(h).WithWidth(w))
		if err != nil {
			debugx.Println("unable to resize tty", err)
		}
	}

	resize()

	go func() {
		for {
			select {
			case <-winCtx.Done():
				return
			case <-winChange:
				resize()
			}
		}
	}()
}
