package shell

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiexec"
)

type execer func(ctx context.Context, dir string, environ []string, cmd string, args []string) error
type entrypoint func(ctx context.Context, user string, group string, cmd string, directory string, environ []string, do execer) (err error)

type Command struct {
	user      string
	group     string
	cmd       string
	directory string
	environ   []string
	timeout   time.Duration
	attempts  int16
	lenient   bool
	entry     entrypoint
	exec      execer
}

// number of attempts to make before giving up.
func (t Command) Attempts(a int16) Command {
	t.attempts = a
	return t
}

// directory to run the command in. must be a relative path.
func (t Command) Directory(d string) Command {
	t.directory = d
	return t
}

// directory to run the command in. must be a relative path.
func (t Command) Lenient(d bool) Command {
	t.lenient = d
	return t
}

// maximum duration for a command to run.
func (t Command) Timeout(d time.Duration) Command {
	t.timeout = d
	return t
}

// append a set of environment variables in the form KEY=VALUE to the environment.
func (t Command) EnvironFrom(environ ...string) Command {
	t.environ = append(t.environ, environ...)
	return t
}

// append a specific key/value environment variable.
func (t Command) Environ(k string, v any) Command {
	switch _v := v.(type) {
	case string:
		t.environ = append(t.environ, fmt.Sprintf("%s=%s", k, _v))
	case int8, int16, int32, int64, int:
		t.environ = append(t.environ, fmt.Sprintf("%s=%d", k, _v))
	default:
		t.environ = append(t.environ, fmt.Sprintf("%s=%v", k, _v))
	}
	return t
}

// user to run the command as
func (t Command) User(u string) Command {
	t.user = u
	return t
}

// group to run the command as
func (t Command) Group(g string) Command {
	t.group = g
	return t
}

// convience function that sets both user and group to the provided value.
func (t Command) As(u string) Command {
	t.user = u
	t.group = u
	return t
}

// specialized for As("root") which runs the command as root.
func (t Command) Privileged() Command {
	return t.As("root")
}

// Internal use only not under compatability promises.
func (t Command) UnsafeEntrypoint(e entrypoint) Command {
	t.entry = e
	return t
}

// New clone the current command configuration and replace the command
// that will be executed.
func (t Command) New(cmd string) Command {
	var (
		environ = make([]string, len(t.environ))
	)

	copy(environ, t.environ)
	d := t
	d.cmd = cmd
	d.environ = environ

	return d
}

// Newf provides a simple printf form of creating commands.
func (t Command) Newf(cmd string, options ...any) Command {
	return t.New(fmt.Sprintf(cmd, options...))
}

// New create a new command with reasonable defaults.
// defaults:
//
//	timeout: 5 minutes.
func New(cmd string) Command {
	return Command{
		user:    "egd", // default user to execute commands as
		group:   "egd",
		cmd:     cmd,
		timeout: 5 * time.Minute,
		entry:   run,
		exec:    ffiexec.Command,
	}
}

// Newf provides a simple printf form of creating commands.
func Newf(cmd string, options ...any) Command {
	return New(fmt.Sprintf(cmd, options...))
}

// Runtime creates a Command with no specified command to run.
// and can be used as a template:
//
// tmp := shell.Runtime().Environ("FOO", "BAR")
//
// shell.Run(
//
//	tmp.New("ls -lha"),
//	tmp.New("echo hello world"),
//
// )
func Runtime() Command {
	return New("")
}

// Convience function for running a set of commands as an operation.
func Op(cmds ...Command) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return Run(ctx, cmds...)
	}
}

// Run the provided commands using the operation.
func Run(ctx context.Context, cmds ...Command) (err error) {
	for _, cmd := range cmds {
		if err = retry(ctx, cmd, func() error {
			cctx, done := context.WithTimeout(ctx, cmd.timeout)
			defer done()

			if cause := cmd.entry(cctx, cmd.user, cmd.group, cmd.cmd, cmd.directory, cmd.environ, cmd.exec); cmd.lenient && cause != nil {
				log.Println("command failed, but lenient mode enable, ignoring", err)
				return nil
			} else if cause != nil {
				return cause
			}

			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func retry(ctx context.Context, c Command, do func() error) (err error) {
	attempts := c.attempts
	switch attempts {
	case 0, 1: // handle zero and single attempt case. 0 attempts makes no sense, so assume 1.
		return do()
	case -1: // unlimited attempts.
		attempts = math.MaxInt16
	default:
	}

	for i := int16(0); i < attempts; i++ {
		if cause := do(); cause == nil {
			return nil
		} else {
			err = errorsx.Compact(err, cause)
		}

		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return err
}

func run(ctx context.Context, user string, group string, cmd string, directory string, environ []string, exec execer) (err error) {
	scmd := []string{"-E", "-H", "-u", user, "-g", group, "bash", "-c", cmd}
	return exec(ctx, directory, environ, "sudo", scmd)
}
