package ffiexec

import (
	"context"
	"io"

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

// Stream runs cmd on the host with its stdio connected to stdin, stdout and stderr, and returns its exit code.
func Stream(ctx context.Context, dir string, environ []string, cmd string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int32, error) {
	cc, err := egunsafe.DialModuleControlSocket(ctx)
	if err != nil {
		return -1, err
	}
	defer cc.Close()

	return execproxy.Run(ctx, execproxy.NewProxyClient(cc), &execproxy.ExecRequest{
		Cmd:         cmd,
		Dir:         dir,
		Arguments:   args,
		Environment: environ,
	}, stdin, stdout, stderr)
}
