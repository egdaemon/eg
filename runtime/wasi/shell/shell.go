package shell

import (
	"context"

	"github.com/james-lawrence/eg/runtime/wasi/internal/host"
	"github.com/pkg/errors"
)

func Run(ctx context.Context, command string) (err error) {
	switch code := host.Command("/bin/bash", []string{"-c", command}); code {
	case 0:
		return nil
	default:
		return errors.Errorf("unable to execute command error code:", code)
	}
}
