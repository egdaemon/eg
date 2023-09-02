package shell

import (
	"context"

	"github.com/james-lawrence/eg/runtime/wasi/internal/ffiexec"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffigraph"
	"github.com/pkg/errors"
)

func Run(ctx context.Context, command string) (err error) {
	if ffigraph.Analysing() {
		return nil
	}

	switch code := ffiexec.Command("/bin/bash", []string{"-c", command}); code {
	case 0:
		return nil
	default:
		return errors.Errorf("unable to execute command error code: %d", code)
	}
}
