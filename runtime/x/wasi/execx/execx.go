package execx

import (
	"context"
	"os/exec"

	"github.com/egdaemon/eg/internal/execx"
)

func String(ctx context.Context, prog string, args ...string) (_ string, err error) {
	return execx.String(ctx, prog, args...)
}

func MaybeRun(c *exec.Cmd) error {
	return execx.MaybeRun(c)
}

func LookPath(name string) (string, error) {
	return execx.LookPath(name)
}
