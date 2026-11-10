// Package ffiegvm asks the host to run a module in a virtual machine whose root is an egiso.
package ffiegvm

import (
	"context"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
)

// Run runs the module at modulepath in a machine whose root is the egiso at location, pinned to toc, the digest of
// its table of contents, with environ added to its environment.
func Run(ctx context.Context, location string, toc string, modulepath string, environ ...string) error {
	locationptr, locationlen := ffiguest.String(location)
	tocptr, toclen := ffiguest.String(toc)
	modulepathptr, modulepathlen := ffiguest.String(modulepath)
	environptr, environlen, environsize := ffiguest.StringArray(environ...)
	return ffiguest.Error(
		run(
			ffiguest.ContextDeadline(ctx),
			locationptr, locationlen,
			tocptr, toclen,
			modulepathptr, modulepathlen,
			environptr, environlen, environsize,
		),
		errorsx.Errorf("unable to run the module in the vm: %s", location),
	)
}
