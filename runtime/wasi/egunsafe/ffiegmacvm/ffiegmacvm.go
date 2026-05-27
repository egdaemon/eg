package ffiegmacvm

import (
	"context"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/macvm"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

func Pull(ctx context.Context, name, image string, options []string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	_, err = macvm.NewProxyClient(cc).Pull(ctx, &macvm.PullRequest{
		Name:    name,
		Image:   image,
		Options: options,
	})
	return errorsx.Wrap(err, "macvm pull failed")
}

func Run(ctx context.Context, name string, cmd, options []string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	_, err = macvm.NewProxyClient(cc).Run(ctx, &macvm.RunRequest{
		Name:    name,
		Command: cmd,
		Options: options,
	})
	return errorsx.Wrap(err, "macvm run failed")
}

func Module(ctx context.Context, name, modulepath string, options []string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	_, err = macvm.NewProxyClient(cc).Module(ctx, &macvm.ModuleRequest{
		Name:    name,
		Module:  modulepath,
		Mdir:    eg.DefaultMountRoot(eg.RuntimeDirectory),
		Options: options,
	})
	return errorsx.Wrap(err, "macvm module failed")
}
