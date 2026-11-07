package egbash

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
)

const (
	DefaultTimeout = 5 * time.Minute
)

// Command is a shell command to interpret, configured like eg/shell's Command.
type Command struct {
	user      string
	group     string
	cmd       string
	directory string
	environ   []string
	timeout   time.Duration
	lenient   bool
}

// directory to run the command in, relative to the working directory.
func (t Command) Directory(d string) Command {
	t.directory = d
	return t
}

// ignore a failure of the command, it is logged instead.
func (t Command) Lenient(d bool) Command {
	t.lenient = d
	return t
}

// maximum duration for a command to run. default is 5 minutes.
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

// user the module runs external commands as.
func (t Command) User(u string) Command {
	t.user = u
	return t
}

// group the module runs external commands as.
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

// specialized for As("root") which runs the command as root when true.
// allows conditional flagging privileged.
// when false the command is unchanged.
func (t Command) MaybePrivileged(b bool) Command {
	if b {
		return t.As("root")
	}
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

// New create a new command with reasonable defaults. unlike eg/shell, external commands run as the module's own
// user until User, Group or As say otherwise.
// defaults:
//
//	timeout: 5 minutes.
func New(cmd string) Command {
	return Command{
		cmd:     cmd,
		timeout: DefaultTimeout,
	}
}

// Newf provides a simple printf form of creating commands.
func Newf(cmd string, options ...any) Command {
	return New(fmt.Sprintf(cmd, options...))
}

// Runtime creates a Command with no specified command to run, for use as a template.
func Runtime() Command {
	return New("")
}

// Env creates a runtime command pre-populated with the current process environment.
func Env() Command {
	return Runtime().EnvironFrom(os.Environ()...)
}

// Run interprets the commands in order, stopping at the first that fails.
func Run(ctx context.Context, cmds ...Command) error {
	for _, cmd := range cmds {
		if err := cmd.run(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (t Command) run(ctx context.Context) error {
	cctx, done := context.WithTimeout(ctx, t.timeout)
	defer done()

	cause := interpret(cctx, t.user, t.group, t.cmd, t.directory, t.environ)
	if t.lenient && cause != nil {
		log.Println("WARN", "command failed, but lenient mode enable, ignoring", cause)
		return nil
	}

	return errorsx.Wrapf(cause, "shell command failed: %s", t.cmd)
}
