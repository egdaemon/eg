package ffiexec

import (
	"context"
	"fmt"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiguest"
)

func Command(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
	dirptr, dirlen := ffiguest.String(dir)
	cmdptr, cmdlen := ffiguest.String(cmd)
	argsptr, argslen, argssize := ffiguest.StringArray(args...)
	envoffset, envlen, envsize := ffiguest.StringArray(environ...)
	return ffiguest.Error(
		command(
			ffiguest.ContextDeadline(ctx),
			dirptr, dirlen,
			envoffset, envlen, envsize,
			cmdptr, cmdlen,
			argsptr, argslen, argssize,
		),
		fmt.Errorf("unable to execute command"),
	)
}
