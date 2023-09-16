package shell

import (
	"context"
	"fmt"
	"time"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiguest"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffiexec"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffigraph"
)

type Command struct {
	cmd       string
	directory string
	environ   []string
	timeout   time.Duration
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

func Run(ctx context.Context, cmds ...Command) (err error) {
	for _, cmd := range cmds {
		if err = run(ctx, cmd); err != nil {
			return err
		}
	}

	return nil
}

func run(ctx context.Context, c Command) (err error) {
	if ffigraph.Analysing() {
		return nil
	}

	cctx, done := context.WithTimeout(ctx, c.timeout)
	defer done()

	cmd := append([]string{"-c"}, c.environ...)
	cmd = append(cmd, c.cmd)
	return ffiguest.Error(ffiexec.Command(ffiguest.ContextDeadline(cctx), c.directory, "/bin/bash", cmd), fmt.Errorf("unable to execute command"))
}
