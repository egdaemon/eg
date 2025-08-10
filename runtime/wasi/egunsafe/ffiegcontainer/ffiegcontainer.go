package ffiegcontainer

import (
	"context"
	"fmt"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

func Pull(ctx context.Context, name string, args []string) error {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	containers := c8s.NewProxyClient(cc)
	_, err = containers.Pull(ctx, &c8s.PullRequest{
		Name:    name,
		Options: args,
	})
	return errorsx.Wrap(err, "pull failed")
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
		Mdir:    eg.DefaultMountRoot(eg.RuntimeDirectory),
		Options: options,
	})
	if err != nil {
		return err
	}

	return nil
}
