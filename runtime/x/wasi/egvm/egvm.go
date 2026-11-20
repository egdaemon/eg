// Package egvm runs modules in a machine booted from an egiso, within the machine running them, in
// place of a container:
//
//	eg.Module(ctx, egvm.New(), ops...)
//	eg.Module(ctx, egvm.New(egvm.OptionEgiso("https://example.com/root.egiso#sha256:…")), ops...)
//
// A machine of a Containerfile is built first, as a container is:
//
//	vm := egvm.New(egvm.OptionContainerfile(".eg/Containerfile"))
//	eg.Perform(ctx, eg.Build(vm), eg.Module(ctx, vm, ops...))
//
// The root module boots it, as it runs containers. Only a module running in a machine, as eg compute
// local --egiso runs one, has an egiso to boot one from by default.
package egvm

import (
	"context"
	"fmt"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegmachine"
)

// VM is the machine implementation of the virtualization layer (eg.C and eg.V).
type VM struct {
	uri           string
	containerfile string
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

// OptionContainerfile boots the machine from an egiso of the image of the Containerfile at path, relative to
// the working directory as eg.C8S.BuildFromFile's is, which CompileWith builds in its directory with buildah.
// The machine running the module keeps the egiso of the image's latest build, so only a changed image is
// converted again.
func OptionContainerfile(path string) Option {
	return func(t *VM) {
		t.containerfile = path
	}
}

// image is the name of the image of the machine's Containerfile, none without one.
func (t VM) image() string {
	if stringsx.Blank(t.containerfile) {
		return ""
	}
	return fmt.Sprintf("egvm.%s", md5x.String(t.containerfile))
}

// CompileWith builds the image of the machine's Containerfile; an egiso needs nothing built.
func (t VM) CompileWith(ctx context.Context) error {
	if stringsx.Blank(t.containerfile) {
		return nil
	}
	return ffiegmachine.Build(ctx, t.image(), t.containerfile, nil)
}

// RunWith runs the module at mpath in the machine.
func (t VM) RunWith(ctx context.Context, mpath string) (err error) {
	return ffiegmachine.Module(ctx, t.uri, t.image(), mpath)
}

// internal use.
func (t VM) ToModuleRunner() VM {
	return t
}
