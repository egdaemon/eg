package shell

import (
	"context"
	"fmt"
	"time"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiguest"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffiexec"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffigraph"
)

type option func(*Command)

type Command struct {
	cmd     string
	timeout time.Duration
}

func (t Command) Timeout(d time.Duration) Command {
	t.timeout = d
	return t
}

// New create a new command with reasonable defaults.
// defaults:
//
//	timeout: 5 minutes.
func New(cmd string, options ...option) Command {
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

	return ffiguest.Error(ffiexec.Command(ffiguest.ContextDeadline(cctx), "/bin/bash", []string{"-c", c.cmd}), fmt.Errorf("unable to execute command"))
}
