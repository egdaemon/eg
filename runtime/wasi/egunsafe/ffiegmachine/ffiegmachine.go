// Package ffiegmachine asks the machines service on the control socket to run a module in a machine.
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

// Module runs the module at modulepath in a machine booted from the egiso uri names, or, blank, from
// the egiso of the machine running the caller.
func Module(ctx context.Context, uri string, modulepath string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	defer cc.Close()

	_, err = machines.NewMachinesClient(cc).Module(ctx, &machines.ModuleRequest{
		Egiso:   uri,
		Module:  modulepath,
		Options: []string{fmt.Sprintf("%s=%d", eg.EnvComputeModuleNestedLevel, envx.Int(-1, eg.EnvComputeModuleNestedLevel)+1)},
	})
	return errorsx.Wrap(err, "unable to run the module in a machine")
}
