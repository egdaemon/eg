package ffiexec

import (
	"context"

	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

func Command(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
	cc, err := egunsafe.DialModuleControlSocket(ctx)
	if err != nil {
		return err
	}
	svc := execproxy.NewProxyClient(cc)

	_, err = svc.Exec(ctx, &execproxy.ExecRequest{
		Cmd:         cmd,
		Dir:         dir,
		Arguments:   args,
		Environment: environ,
	})

	return err
}
