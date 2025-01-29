package ffiegcontainer

import (
	"context"
	"fmt"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
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
	nameptr, namelen := ffiguest.String(name)
	defptr, deflen := ffiguest.String(definition)
	argsptr, argslen, argssize := ffiguest.StringArray(args...)
	return ffiguest.Error(
		build(
			ffiguest.ContextDeadline(ctx),
			nameptr, namelen,
			defptr, deflen,
			argsptr, argslen, argssize,
		),
		fmt.Errorf("build failed"),
	)
}

func Run(ctx context.Context, name, modulepath string, cmd []string, args []string) error {
	nameptr, namelen := ffiguest.String(name)
	mpathptr, mpathlen := ffiguest.String(modulepath)
	cmdptr, cmdlen, cmdsize := ffiguest.StringArray(cmd...)
	argsptr, argslen, argssize := ffiguest.StringArray(args...)
	return ffiguest.Error(
		run(
			ffiguest.ContextDeadline(ctx),
			nameptr, namelen,
			mpathptr, mpathlen,
			cmdptr, cmdlen, cmdsize,
			argsptr, argslen, argssize,
		),
		fmt.Errorf("run failed"),
	)
}

func Module(ctx context.Context, name, modulepath string, options []string) error {
	nameptr, namelen := ffiguest.String(name)
	mpathptr, mpathlen := ffiguest.String(modulepath)
	argsptr, argslen, argssize := ffiguest.StringArray(options...)

	return ffiguest.Error(
		module(
			ffiguest.ContextDeadline(ctx),
			nameptr, namelen,
			mpathptr, mpathlen,
			argsptr, argslen, argssize,
		),
		fmt.Errorf("module failed"),
	)
	// cc, err := egunsafe.DialControlSocket(ctx)
	// if err != nil {
	// 	return err
	// }
	// containers := c8s.NewProxyClient(cc)

	// cname := fmt.Sprintf("%s.%s", name, md5x.String(modulepath+envx.String(eg.EnvComputeRunID)))
	// options = append(
	// 	options,
	// 	"--volume", fmt.Sprintf("%s:%s:ro", filepath.Join(eg.DefaultMountRoot(eg.RuntimeDirectory), modulepath), eg.DefaultMountRoot(eg.ModuleBin)),
	// )

	// _, err = containers.Module(ctx, &c8s.ModuleRequest{
	// 	Image:   name,
	// 	Name:    cname,
	// 	Mdir:    eg.DefaultMountRoot(eg.RuntimeDirectory),
	// 	Options: options,
	// })
	// if err != nil {
	// 	return err
	// }

	// return nil
}
