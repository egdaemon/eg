// Package ffiegvm asks the host to run a module in a machine whose root is an egiso.
package ffiegvm

import (
	"context"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
)

// Run runs the module at modulepath in a machine booted from the egiso uri names, the running
// machine's own when blank, with environ added to its environment.
func Run(ctx context.Context, uri string, modulepath string, environ ...string) error {
	uriptr, urilen := ffiguest.String(uri)
	modulepathptr, modulepathlen := ffiguest.String(modulepath)
	environptr, environlen, environsize := ffiguest.StringArray(environ...)
	return ffiguest.Error(
		run(
			ffiguest.ContextDeadline(ctx),
			uriptr, urilen,
			modulepathptr, modulepathlen,
			environptr, environlen, environsize,
		),
		errorsx.Errorf("unable to run the module in a machine: %s", uri),
	)
}
