package ffiegcontainer

import (
	"context"
	"fmt"
	"sync"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegmachine"
)

func Pull(ctx context.Context, name string, args []string) error {
	nameptr, namelen := ffiguest.String(name)
	argsptr, argslen, argssize := ffiguest.StringArray(args...)
	return ffiguest.Error(
		pull(
			ffiguest.ContextDeadline(ctx),
			nameptr, namelen,
			argsptr, argslen, argssize,
		),
		fmt.Errorf("pull failed"),
	)
}

// containerfiles are the Containerfiles of the containers a module running in a machine built, by name.
var containerfiles sync.Map

// machine reports whether the modules of the container name run in a machine, and the Containerfile its egiso
// is built from. A module running in a machine runs the modules of the containers it built in machines: the
// root container, eg, in one booted from the machine's own egiso, and any other in one booted from an egiso of
// its Containerfile. A container it pulled runs in a container.
func machine(name string) (containerfile string, ok bool) {
	var (
		built any
	)

	if stringsx.Blank(envx.String("", eg.EnvComputeEgiso)) {
		return "", false
	}
	if name == "eg" {
		return "", true
	}
	if built, ok = containerfiles.Load(name); !ok {
		return "", false
	}
	return built.(string), true
}

// Build builds the container name from definition. A module running in a machine only records definition: the
// machine its modules run in builds it.
func Build(ctx context.Context, name, definition string, args []string) error {
	if stringsx.Present(envx.String("", eg.EnvComputeEgiso)) {
		containerfiles.Store(name, definition)
		return nil
	}

	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	containers := c8s.NewProxyClient(cc)
	_, err = containers.Build(ctx, &c8s.BuildRequest{
		Name:       name,
		Definition: definition,
		Options:    args,
	})
	return errorsx.Wrap(err, "build failed")
}

func Run(ctx context.Context, name, modulepath string, cmd []string, args []string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	containers := c8s.NewProxyClient(cc)

	_, err = containers.Run(ctx, &c8s.RunRequest{
		Image:   name,
		Name:    fmt.Sprintf("%s.%s", name, md5x.String(modulepath+envx.String(eg.EnvComputeRunID))),
		Command: cmd,
		Options: args,
	})
	return errorsx.Wrap(err, "fun failed")
}

// Module runs the module at modulepath in the container name, or in a machine, see machine.
func Module(ctx context.Context, name, modulepath string, options []string) error {
	if containerfile, ok := machine(name); ok {
		return ffiegmachine.Module(ctx, "", containerfile, modulepath)
	}

	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	containers := c8s.NewProxyClient(cc)

	cname := fmt.Sprintf("%s.%s", name, md5x.String(modulepath+envx.String("", eg.EnvComputeRunID)))

	_, err = containers.Module(ctx, &c8s.ModuleRequest{
		Image:   name,
		Name:    cname,
		Module:  modulepath,
		Mdir:    egunsafe.RuntimeDirectory(),
		Options: options,
	})
	if err != nil {
		return err
	}

	return nil
}
