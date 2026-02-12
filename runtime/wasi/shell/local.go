package shell

import (
	"context"
	"io"
	"os"
	"os/exec"
)

type local struct {
	stdout io.Writer
	stderr io.Writer
}

func (t local) execlocal(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), environ...)
	c.Stdout = t.stdout
	c.Stderr = t.stderr
	return c.Run()
}

// NewLocal creates a command that executes locally via bash
// without the WASI runtime or sudo. Useful for testing.
func NewLocal() Command {
	return NewLocalStd(os.Stdout, os.Stderr)
}

func NewLocalStd(stdout, stderr io.Writer) Command {
	l := local{stdout: stdout, stderr: stderr}
	return Command{
		timeout:  DefaultTimeout,
		entry:    runlocal,
		exec:     l.execlocal,
		attempts: 1,
	}
}

func runlocal(ctx context.Context, user string, group string, cmd string, directory string, environ []string, do execer) (err error) {
	return do(ctx, directory, environ, "bash", []string{"-c", cmd})
}
