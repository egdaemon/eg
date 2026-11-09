package eg

import (
	"context"
	"fmt"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegcontainer"
)

// Cell runs modules with the egiso at location, an https URL or a path on the host, as their root
// in place of a container. toc, the digest of the egiso's table of contents, pins it: the host
// refuses an egiso whose table of contents does not match, and checks every chunk it reads
// against that table. Changes a module makes to the root last for that module's run.
func Cell(location string, toc string) CellRuntime {
	return CellRuntime{location: location, toc: toc}
}

// CellRuntime is the egiso implementation of the virtualization layer (C and V).
type CellRuntime struct {
	location string
	toc      string
}

// CompileWith prepares nothing: the host opens and verifies the egiso when a module runs in it.
func (t CellRuntime) CompileWith(ctx context.Context) error {
	return nil
}

// RunWith runs the module at mpath in the cell.
func (t CellRuntime) RunWith(ctx context.Context, mpath string) error {
	level := fmt.Sprintf("%s=%d", eg.EnvComputeModuleNestedLevel, envx.Int(-1, eg.EnvComputeModuleNestedLevel)+1)
	return ffiegcontainer.Cell(ctx, t.location, t.toc, mpath, []string{"-e", level})
}

// internal use.
func (t CellRuntime) ToModuleRunner() CellRuntime {
	return t
}
