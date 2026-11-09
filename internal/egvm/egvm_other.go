//go:build !darwin

package egvm

import (
	"context"
	"io"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy"
)

// VM is a running machine.
type VM struct{}

// Boot fails: machines need Virtualization.framework.
func Boot(m Machine, console io.Writer) (*VM, error) {
	return nil, errorsx.String("egvm machines need macOS")
}

// Exec fails: machines need Virtualization.framework.
func (t *VM) Exec(ctx context.Context, req *execproxy.ExecRequest, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int32, error) {
	return -1, errorsx.String("egvm machines need macOS")
}

// Close does nothing.
func (t *VM) Close() error {
	return nil
}

// Mount fails: only macOS mounts NFS without privileges.
func Mount(fsys *egiso.SysFS, dir string) (unmount func() error, err error) {
	return nil, errorsx.String("egvm machines need macOS")
}
