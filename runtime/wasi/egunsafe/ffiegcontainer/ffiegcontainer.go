package ffiegcontainer

import (
	"context"
	"fmt"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
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

func Build(ctx context.Context, name, definition string, args []string) error {
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

func Module(ctx context.Context, name, modulepath string, options []string) error {
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

// Cell runs the module at modulepath with the egiso at location, pinned to toc, the digest of its
// table of contents, as its root in place of a container.
func Cell(ctx context.Context, location, toc, modulepath string, options []string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	containers := c8s.NewProxyClient(cc)

	_, err = containers.Module(ctx, &c8s.ModuleRequest{
		Image:   location,
		Toc:     toc,
		Module:  modulepath,
		Mdir:    egunsafe.RuntimeDirectory(),
		Options: options,
	})
	return errorsx.Wrapf(err, "unable to run the module in the cell: %s", location)
}
