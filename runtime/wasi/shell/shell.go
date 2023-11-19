package shell

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffiexec"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffigraph"
)

type Command struct {
	cmd       string
	directory string
	environ   []string
	timeout   time.Duration
	attempts  int16
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

// maximum duration for a command to run.
func (t Command) Timeout(d time.Duration) Command {
	t.timeout = d
	return t
}

func (t Command) Environ(k, v string) Command {
	t.environ = append(t.environ, fmt.Sprintf("%s=\"%s\"", k, v))
	return t
}

// New clone the current command configuration and replace the command
// that will be executed.
func (t Command) New(cmd string) Command {
	var (
		environ []string
	)

	copy(environ, t.environ)
	d := t
	d.cmd = cmd
	d.environ = environ

	return d
}

// New create a new command with reasonable defaults.
// defaults:
//
//	timeout: 5 minutes.
func New(cmd string) Command {
	return Command{
		cmd:     cmd,
		timeout: 5 * time.Minute,
	}
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
	return Command{
		timeout: 5 * time.Minute,
	}
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

	}

	return err
}

func run(ctx context.Context, c Command) (err error) {
	if ffigraph.Analysing() {
		return nil
	}

	cctx, done := context.WithTimeout(ctx, c.timeout)
	defer done()

	cmd := append([]string{"-c"}, c.environ...)
	cmd = append(cmd, c.cmd)
	return ffiexec.Command(cctx, c.directory, "bash", cmd)
}
