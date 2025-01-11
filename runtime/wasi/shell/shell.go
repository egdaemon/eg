package shell

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiexec"
)

type Command struct {
	user      string
	group     string
	cmd       string
	directory string
	environ   []string
	timeout   time.Duration
	attempts  int16
	lenient   bool
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

// shorthand for As("") which runs the command as root.
func (t Command) Privileged() Command {
	t.user = "root"
	t.group = "root"
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
		group:   "root",
		cmd:     cmd,
		timeout: 5 * time.Minute,
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

func Run(ctx context.Context, cmds ...Command) (err error) {
	for _, cmd := range cmds {
		if err = retry(ctx, cmd, func() error { return run(ctx, cmd) }); err != nil {
			return err
		}
	}

	return nil
}

func retry(ctx context.Context, c Command, do func() error) (err error) {
	retries := c.attempts
	switch retries {
	case 1:
		return do()
	case 0:
		return do()
	case -1:
		retries = math.MaxInt16
	}

	for i := int16(0); i < retries; i++ {
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

func run(ctx context.Context, c Command) (err error) {
	cctx, done := context.WithTimeout(ctx, c.timeout)
	defer done()

	cmd := []string{"-c", stringsx.Join(" ", "sudo", "-E", "-H", "-g", c.group, "-u", c.user, c.cmd)}

	err = ffiexec.Command(cctx, c.directory, c.environ, "bash", cmd)
	if c.lenient && err != nil {
		log.Println("command failed, but lenient mode enable", err)
		return nil
	}

	return err
}
