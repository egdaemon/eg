// Package egvm runs modules in a virtual machine whose root is an egiso, in place of a container:
//
//	eg.Module(ctx, egvm.New("https://example.com/root.egiso", "sha256:…"), ops...)
package egvm

import (
	"context"
	"fmt"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegvm"
)

// New runs modules with the egiso at location, an https URL or a path on the host, as their root. toc, the
// digest of the egiso's table of contents, pins it: the host refuses an egiso whose table of contents does not
// match, and checks every chunk it reads against that table. Changes a module makes to the root last for that
// module's run.
func New(location string, toc string) VM {
	return VM{location: location, toc: toc}
}

// VM is the egiso implementation of the virtualization layer (eg.C and eg.V).
type VM struct {
	location string
	toc      string
}

// CompileWith prepares nothing: the host opens and verifies the egiso when a module runs in it.
func (t VM) CompileWith(ctx context.Context) error {
	return nil
}

// RunWith runs the module at mpath in the machine.
func (t VM) RunWith(ctx context.Context, mpath string) error {
	level := fmt.Sprintf("%s=%d", eg.EnvComputeModuleNestedLevel, envx.Int(-1, eg.EnvComputeModuleNestedLevel)+1)
	return ffiegvm.Run(ctx, t.location, t.toc, mpath, level)
}

// internal use.
func (t VM) ToModuleRunner() VM {
	return t
}
