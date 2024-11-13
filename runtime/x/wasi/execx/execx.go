package execx

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/egdaemon/eg/internal/execx"
)

func String(ctx context.Context, prog string, args ...string) (_ string, err error) {
	var (
		buf bytes.Buffer
	)

	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stdout = &buf

	if err = cmd.Run(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func MaybeRun(c *exec.Cmd) error {
	return execx.MaybeRun(c)
}

func LookPath(name string) (string, error) {
	return execx.LookPath(name)
}
