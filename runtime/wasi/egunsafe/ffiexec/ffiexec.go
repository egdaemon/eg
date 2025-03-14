package ffiexec

import (
	"context"

	"github.com/egdaemon/eg/interp/exec"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

func Command(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
	cc, err := egunsafe.DialModuleControlSocket(ctx)
	if err != nil {
		return err
	}
	svc := exec.NewProxyClient(cc)

	_, err = svc.Exec(ctx, &exec.ExecRequest{
		Cmd:         cmd,
		Dir:         dir,
		Arguments:   args,
		Environment: environ,
	})

	return err

	// dirptr, dirlen := ffiguest.String(dir)
	// cmdptr, cmdlen := ffiguest.String(cmd)
	// argsptr, argslen, argssize := ffiguest.StringArray(args...)
	// envoffset, envlen, envsize := ffiguest.StringArray(environ...)
	// return ffiguest.Error(
	// 	command(
	// 		ffiguest.ContextDeadline(ctx),
	// 		dirptr, dirlen,
	// 		envoffset, envlen, envsize,
	// 		cmdptr, cmdlen,
	// 		argsptr, argslen, argssize,
	// 	),
	// 	fmt.Errorf("unable to execute command: %s", cmd),
	// )
}
