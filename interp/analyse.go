package interp

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/james-lawrence/eg/internal/md5x"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiegcontainer"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiexec"
	"github.com/tetratelabs/wazero"
)

func Analyse(ctx context.Context, runid, dir string, module string, options ...Option) error {
	var (
		r = runner{
			root:      dir,
			moduledir: ".eg",
			initonce:  &sync.Once{},
		}
	)

	for _, opt := range options {
		opt(&r)
	}

	runtimeenv := func(r runner, moduledir string, cmdenv []string, host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
		return host.NewFunctionBuilder().WithFunc(ffiegcontainer.Build(func(ctx context.Context, name, definition string) (cmd *exec.Cmd, err error) {
			// cmd, err = ffiegcontainer.PodmanBuild(ctx, name, moduledir, definition)
			// cmd.Dir = r.root
			// cmd.Env = cmdenv
			// cmd.Stderr = os.Stderr
			// cmd.Stdout = os.Stdout
			// return cmd, err
			return nil, nil
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Build").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Run(func(ctx context.Context, name, modulepath string) (err error) {
			cmdctx := func(cmd *exec.Cmd) *exec.Cmd {
				return nil
				// cmd.Dir = r.root
				// cmd.Env = cmdenv
				// cmd.Stderr = os.Stderr
				// cmd.Stdout = os.Stdout
				// return cmd
			}
			cname := fmt.Sprintf("%s.%s", name, md5x.DigestString(modulepath+runid))
			return ffiegcontainer.PodmanRun(ctx, cmdctx, name, cname, r.root, r.moduledir, modulepath, "/home/james.lawrence/go/bin/eg")
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Run").
			NewFunctionBuilder().WithFunc(ffiexec.Exec(func(cmd *exec.Cmd) *exec.Cmd {
			return nil
			// cmd.Dir = r.root
			// cmd.Env = cmdenv
			// cmd.Stderr = os.Stderr
			// cmd.Stdout = os.Stdout
			// return cmd
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiexec.Command")
	}

	return r.perform(ctx, runid, module, runtimeenv)
}
