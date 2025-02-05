package ffiexec

import (
	"context"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/exec"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

func Command(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	svc := exec.NewProxyClient(cc)

	resp, err := svc.Exec(ctx, &exec.ExecRequest{
		Dir: dir,
	})
	if err != nil {
		return err
	}

	if resp.Errcode != 0 {
		return errorsx.Errorf("unable to exec command: %d - %s", resp.Errcode, cmd)
	}

	return nil
}
