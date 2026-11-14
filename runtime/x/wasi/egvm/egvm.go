// Package egvm runs modules in a machine booted from an egiso, beside the machine running them, in
// place of a container:
//
//	eg.Module(ctx, egvm.New(), ops...)
//	eg.Module(ctx, egvm.New(egvm.OptionEgiso("https://example.com/root.egiso#sha256:…")), ops...)
//
// Only a module running in a machine, as eg compute local --egiso runs one, can boot another.
package egvm

import (
	"context"
	"fmt"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegvm"
)

// VM is the machine implementation of the virtualization layer (eg.C and eg.V).
type VM struct {
	uri string
}

// Option changes the machine New describes.
type Option func(*VM)

// New runs modules in a machine booted from the egiso of the machine running them. Changes a module
// makes to the machine's root last for that module's run; it shares the workspace's directories.
func New(options ...Option) VM {
	return langx.Clone(VM{}, options...)
}

// OptionEgiso boots the machine from the egiso uri names: an https URL or a path on the host whose
// fragment, the digest of the egiso's table of contents, pins it. The host refuses an egiso whose
// table of contents does not match, and checks every chunk it reads against that table.
func OptionEgiso(uri string) Option {
	return func(t *VM) {
		t.uri = uri
	}
}

// CompileWith prepares nothing: the host opens and verifies the egiso when a module runs in it.
func (t VM) CompileWith(ctx context.Context) error {
	return nil
}

// RunWith runs the module at mpath in the machine.
func (t VM) RunWith(ctx context.Context, mpath string) error {
	level := fmt.Sprintf("%s=%d", eg.EnvComputeModuleNestedLevel, envx.Int(-1, eg.EnvComputeModuleNestedLevel)+1)
	return ffiegvm.Run(ctx, t.uri, mpath, level)
}

// internal use.
func (t VM) ToModuleRunner() VM {
	return t
}
