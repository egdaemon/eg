package shellx

import (
	"bytes"
	"context"
	"os/exec"
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
