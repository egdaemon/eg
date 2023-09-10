package interp

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/awalterschulze/gographviz"
	"github.com/james-lawrence/eg/internal/md5x"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiegcontainer"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiexec"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffigraph"
	"github.com/tetratelabs/wazero"
)

func Analyse(ctx context.Context, runid, dir string, module string, options ...Option) (err error) {
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

	gg := gographviz.NewGraph()
	gg.Directed = true

	runtimeenv := func(r runner, moduledir string, cmdenv []string, host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
		g := ffigraph.NewViz(gg)
		return host.NewFunctionBuilder().
			WithFunc(ffigraph.Analysing(true)).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Analysing").
			NewFunctionBuilder().
			WithFunc(g.Pusher()).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Push").
			NewFunctionBuilder().
			WithFunc(g.Popper()).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Pop").
			NewFunctionBuilder().
			WithFunc(ffiegcontainer.Pull(func(ctx context.Context, name string, options ...string) (cmd *exec.Cmd, err error) {
				// cmd, err = ffiegcontainer.PodmanBuild(ctx, name, moduledir, definition)
				// cmd.Dir = r.root
				// cmd.Env = cmdenv
				// cmd.Stderr = os.Stderr
				// cmd.Stdout = os.Stdout
				// return cmd, err
				return nil, nil
			})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Pull").
			NewFunctionBuilder().
			WithFunc(ffiegcontainer.Build(func(ctx context.Context, name, definition string, options ...string) (cmd *exec.Cmd, err error) {
				// cmd, err = ffiegcontainer.PodmanBuild(ctx, name, moduledir, definition)
				// cmd.Dir = r.root
				// cmd.Env = cmdenv
				// cmd.Stderr = os.Stderr
				// cmd.Stdout = os.Stdout
				// return cmd, err
				return nil, nil
			})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Build").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Run(func(ctx context.Context, name, modulepath string, cmd []string, options ...string) (err error) {
			cmdctx := func(cmd *exec.Cmd) *exec.Cmd {
				return nil
				// cmd.Dir = r.root
				// cmd.Env = cmdenv
				// cmd.Stderr = os.Stderr
				// cmd.Stdout = os.Stdout
				// return cmd
			}
			cname := fmt.Sprintf("%s.%s", name, md5x.DigestString(modulepath+runid))

			options = append(
				options,
				"-w", r.moduledir,
				"--volume", fmt.Sprintf("%s:/opt/eg:O", r.root),
			)

			return ffiegcontainer.PodmanRun(ctx, cmdctx, name, cname, cmd, options...)
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Run").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Module(func(ctx context.Context, name, modulepath string, options ...string) (err error) {
			cmdctx := func(cmd *exec.Cmd) *exec.Cmd {
				return nil
				// cmd.Dir = r.root
				// cmd.Env = cmdenv
				// cmd.Stderr = os.Stderr
				// cmd.Stdout = os.Stdout
				// return cmd
			}
			cname := fmt.Sprintf("%s.%s", name, md5x.DigestString(runid))
			return ffiegcontainer.PodmanModule(ctx, cmdctx, name, cname, r.moduledir, options...)
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Module").
			NewFunctionBuilder().WithFunc(ffiexec.Exec(func(cmd *exec.Cmd) *exec.Cmd {
			return nil
			// cmd.Dir = r.root
			// cmd.Env = cmdenv
			// cmd.Stderr = os.Stderr
			// cmd.Stdout = os.Stdout
			// return cmd
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiexec.Command")
	}

	if err = r.perform(ctx, runid, module, runtimeenv); err != nil {
		return err
	}

	// if err = os.WriteFile(runid+".dot", []byte(gg.String()), 0600); err != nil {
	// 	return err
	// }

	return nil
}
