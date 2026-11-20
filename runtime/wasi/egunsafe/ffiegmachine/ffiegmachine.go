// Package ffiegmachine asks the machines service on the control socket to build images and run modules in machines.
package ffiegmachine

import (
	"context"
	"fmt"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/machines"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

// Build builds the image name from the Containerfile at definition, as ffiegcontainer.Build builds a container.
func Build(ctx context.Context, name, definition string, options []string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	defer cc.Close()

	_, err = machines.NewMachinesClient(cc).Build(ctx, &machines.BuildRequest{
		Name:       name,
		Definition: definition,
		Options:    options,
	})
	return errorsx.Wrap(err, "build failed")
}

// Module runs the module at modulepath in a machine booted from an egiso of the image Build built, else from the
// egiso uri names, else, both blank, from the egiso of the machine running the caller.
func Module(ctx context.Context, uri string, image string, modulepath string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	defer cc.Close()

	_, err = machines.NewMachinesClient(cc).Module(ctx, &machines.ModuleRequest{
		Egiso:   uri,
		Image:   image,
		Module:  modulepath,
		Options: []string{fmt.Sprintf("%s=%d", eg.EnvComputeModuleNestedLevel, envx.Int(-1, eg.EnvComputeModuleNestedLevel)+1)},
	})
	return errorsx.Wrap(err, "unable to run the module in a machine")
}
