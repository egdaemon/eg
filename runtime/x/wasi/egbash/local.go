package egbash

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/egdaemon/sh/v3/interp"
	"github.com/logrusorgru/aurora"
)

// NewLocal creates a command that the interpreter runs locally, starting external commands as processes
// without the WASI runtime or the reactor. Useful for testing.
func NewLocal() Command {
	return Command{
		timeout:    DefaultTimeout,
		entry:      interpret,
		exec:       execlocal,
		attempts:   1,
		retrylimit: defaultRetryLimiter(),
		color:      aurora.NewAurora(false),
	}
}

// execlocal runs cmd as a process with the interpreter's stdio, reporting a non-zero exit as the interpreter's
// exit status so || and $? see it.
func execlocal(ctx context.Context, dir string, environ []string, cmd string, args []string) (err error) {
	var (
		hc = interp.HandlerCtx(ctx)
		c  = exec.CommandContext(ctx, cmd, args...)
	)

	c.Dir = dir
	c.Env = append(os.Environ(), environ...)
	c.Stdin, c.Stdout, c.Stderr = hc.Stdin, hc.Stdout, hc.Stderr
	if err = c.Run(); err == nil {
		return nil
	}

	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return interp.ExitStatus(exit.ExitCode())
	}

	return err
}
