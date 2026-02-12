package shell

import (
	"context"
	"os"
	"os/exec"
)

// NewLocal creates a command that executes locally via bash
// without the WASI runtime or sudo. Useful for testing.
func NewLocal() Command {
	return Command{
		timeout:  DefaultTimeout,
		entry:    runlocal,
		exec:     execlocal,
		attempts: 1,
	}
}

func runlocal(ctx context.Context, user string, group string, cmd string, directory string, environ []string, do execer) (err error) {
	return do(ctx, directory, environ, "bash", []string{"-c", cmd})
}

func execlocal(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), environ...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
