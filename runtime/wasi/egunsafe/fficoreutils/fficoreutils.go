package fficoreutils

import (
	"context"
	"fmt"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
)

// ExitError is returned when the utility exits with a non zero code.
type ExitError struct {
	Name string
	Code int
}

func (t ExitError) Error() string {
	return fmt.Sprintf("%s exited with code %d", t.Name, t.Code)
}

// Run executes the coreutils utility with the given arguments (argv[0] is supplied by the host).
// the utility uses this module's stdio, which the host wires up when it runs the module.
func Run(ctx context.Context, name string, args ...string) error {
	nameptr, namelen := ffiguest.String(name)
	argsptr, argslen, argssize := ffiguest.StringArray(args...)

	code := run(ffiguest.ContextDeadline(ctx), nameptr, namelen, argsptr, argslen, argssize)
	if code == 0 {
		return nil
	}

	return ExitError{Name: name, Code: int(code)}
}
