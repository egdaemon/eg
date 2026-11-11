//go:build !darwin

package egvm

import (
	"context"
	"io"

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
func (t *VM) Exec(ctx context.Context, req *execproxy.ExecRequest, options ...ExecOption) error {
	return errorsx.String("egvm machines need macOS")
}

// Close does nothing.
func (t *VM) Close() error {
	return nil
}
