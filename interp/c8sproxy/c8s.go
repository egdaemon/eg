package c8sproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/termx"
	"github.com/egdaemon/eg/internal/ttyx"
	"github.com/moby/moby/api/types/container"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/errorhandling"

	xterm "golang.org/x/term"
)

func silence(c *exec.Cmd) *exec.Cmd {
	c.Stdout = io.Discard
	return c
}

func cleanup(ctx context.Context, cmdctx func(*exec.Cmd) *exec.Cmd, cname string) {
	// graceful stop first. podman stop waits 10s for SIGTERM before it sends SIGKILL,
	// so give it its own budget with headroom, otherwise the kill races the context.
	sctx, sdone := context.WithTimeout(ctx, 15*time.Second)
	defer sdone()

	// don't care about this error; if the container doesn't exist its fine. a failed
	// stop must not skip the removal below, that is what leaks the published ports.
	if err := execx.MaybeRun(silence(cmdctx(exec.CommandContext(sctx, "podman", "stop", cname)))); err != nil {
		debugx.Println(errorsx.Wrap(err, "container stop failed"))
	}

	// always remove, with a fresh budget. --force covers a container stop could not
	// stop, --ignore tolerates it already being gone. removal tears down the container's
	// pasta process and releases its published ports.
	rctx, rdone := context.WithTimeout(ctx, 10*time.Second)
	defer rdone()

	if err := execx.MaybeRun(silence(cmdctx(exec.CommandContext(rctx, "podman", "rm", "--force", "--ignore", cname)))); err != nil {
		debugx.Println(errorsx.Wrap(err, "container rm failed"))
	}
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

// PodmanModule runs the module within a new container of image, args are the module's os.Args[1:].
func PodmanModule(ctx context.Context, cmdctx func(*exec.Cmd) *exec.Cmd, image, cname, moduledir string, args []string, options ...string) (err error) {
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

	stdin, stdout, stderr := cmd.Stdin, cmd.Stdout, cmd.Stderr
	if termx.IsTTY(stdin, stdout) {
		// the detached run prints the id of the container, the terminal is the module's.
		cmd.Stdout = io.Discard
	}

	if err = execx.MaybeRun(cmd); err != nil {
		return errorsx.Wrap(err, "unable to run container")
	}

	if err = moduleExec(ctx, cname, moduledir, args, stdin, stdout, stderr); err != nil {
		return errorsx.Wrap(err, "unable to exec module")
	}

	return nil
}

func PodmanModuleRunCmd(image, cname string, options ...string) []string {
	args := make([]string, 0, len(options)+11)
	args = append(args,
		// "--log-level", "debug",
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

// modulecmd is the command that runs the module within the container. the arguments of the module follow a --, which
// ends the flags of the command, so the module's flags are not taken for its own, e.g.) -v.
func modulecmd(egbin, verbosity, moduledir string, args []string) []string {
	cmd := slicesx.Filter(stringsx.Present, []string{
		egbin,
		verbosity,
		"module",
		"--directory", eg.DefaultWorkingDirectory(),
		"--moduledir", moduledir,
		eg.ModuleMount(),
	}...)

	if len(args) == 0 {
		return cmd
	}

	return append(append(cmd, "--"), args...)
}

func moduleExec(ctx context.Context, cname, moduledir string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (err error) {
	var (
		rtty *io.PipeReader
		wtty *io.PipeWriter
	)

	// a terminal on both ends is given to the module as one, the files are attached as they are so the terminal is put in raw mode
	// and resized with the window, see execAttach.
	tty := termx.IsTTY(stdin, stdout)

	verbosity := ""
	switch x := envx.Int(0, eg.EnvComputeLoggingVerbosity); x {
	case 0:
	default:
		verbosity = fmt.Sprintf("-%s", strings.Repeat("v", x))
	}

	id, err := containers.ExecCreate(ctx, cname, &handlers.ExecCreateConfig{
		ExecCreateRequest: container.ExecCreateRequest{
			Tty:          tty,
			AttachStdin:  stdin != nil,
			AttachStderr: true,
			AttachStdout: true,
			Cmd:          modulecmd(envx.String("eg", eg.EnvComputeBin), verbosity, moduledir, args),
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

		errorsx.Log(errorsx.Wrap(containers.ExecRemove(ctx, id, &containers.ExecRemoveOptions{Force: new(true)}), "failed to remove exec session"))
	}()

	if stdin != nil && !tty {
		rtty, wtty = io.Pipe()
		defer rtty.Close()
		defer wtty.Close()

		go func() {
			_, cause := io.Copy(wtty, stdin)
			debugx.Println("unable to copy stdin", cause)
			wtty.CloseWithError(cause)
		}()
	}

	attached := io.Reader(rtty)
	if rtty == nil {
		attached = stdin
	}

	if err = execAttach(ctx, id, attached, stdout, stderr); err != nil {
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
		// output processing stays on: anything else writing to the terminal while the module is attached, e.g.) another
		// command or the log, still returns to the first column. see ttyx.Raw.
		restore, err := ttyx.Raw(file)
		if err != nil {
			return err
		}
		defer restore()
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
			_, err := io.Copy(socket, stdin)
			if errorsx.Ignore(err, io.ErrClosedPipe) != nil {
				log.Printf("failed to write input to service: %T - %v\n", err, err)
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
				return errorsx.Ignore(err, io.EOF, io.ErrUnexpectedEOF)
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
