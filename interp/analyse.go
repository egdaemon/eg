package interp

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiegcontainer"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiexec"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigit"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigraph"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffimetric"
	"github.com/egdaemon/eg/runners"
	"github.com/tetratelabs/wazero"
)

func Analyse(ctx context.Context, g ffigraph.Eventer, aid, runid, dir string, module string, options ...Option) (err error) {
	var (
		r = runner{
			root:       dir,
			runtimedir: runners.DefaultRunnerDirectory(runid),
			initonce:   &sync.Once{},
		}
	)

	for _, opt := range options {
		opt(&r)
	}

	runtimeenv := func(r runner, cmdenv []string, host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
		return host.NewFunctionBuilder().
			WithFunc(ffigraph.Analysing(true)).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Analysing").
			NewFunctionBuilder().
			WithFunc(g.Pusher()).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Push").
			NewFunctionBuilder().
			WithFunc(g.Popper()).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Pop").
			NewFunctionBuilder().
			WithFunc(ffiegcontainer.Pull(func(ctx context.Context, name, wdir string, options ...string) (err error) {
				// cmd, err = ffiegcontainer.PodmanBuild(ctx, name, moduledir, definition)
				// cmd.Dir = r.root
				// cmd.Env = cmdenv
				// cmd.Stderr = os.Stderr
				// cmd.Stdout = os.Stdout
				// return cmd, err
				return nil
			})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Pull").
			NewFunctionBuilder().
			WithFunc(ffiegcontainer.Build(func(ctx context.Context, name, wdir, definition string, options ...string) (err error) {
				// cmd, err = ffiegcontainer.PodmanBuild(ctx, name, moduledir, definition)
				// cmd.Dir = r.root
				// cmd.Env = cmdenv
				// cmd.Stderr = os.Stderr
				// cmd.Stdout = os.Stdout
				// return cmd, err
				return nil
			})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Build").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Run(func(ctx context.Context, name, modulepath string, cmd []string, options ...string) (err error) {
			cmdctx := func(cmd *exec.Cmd) *exec.Cmd {
				return nil
				// cmd.Dir = r.root
				// cmd.Env = cmdenv
				// cmd.Stderr = os.Stderr
				// cmd.Stdout = os.Stdout
				// return cmd
			}
			cname := fmt.Sprintf("%s.%s", name, md5x.String(modulepath+runid))

			options = append(
				options,
				"-w", r.root,
				"--volume", fmt.Sprintf("%s:/opt/eg:O", r.root),
			)

			return c8s.PodmanRun(ctx, cmdctx, fmt.Sprintf("%s.%s", aid, name), cname, cmd, options...)
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Run").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Module(func(ctx context.Context, name, modulepath string, options ...string) (err error) {
			cmdctx := func(cmd *exec.Cmd) *exec.Cmd {
				return nil
				// cmd.Dir = r.root
				// cmd.Env = cmdenv
				// cmd.Stderr = os.Stderr
				// cmd.Stdout = os.Stdout
				// return cmd
			}
			cname := fmt.Sprintf("%s.%s", name, md5x.String(runid))
			options = append(
				options,
				"--volume", fmt.Sprintf("%s:%s:O", r.runtimedir, eg.DefaultRuntimeDirectory),
			)
			return c8s.PodmanModule(ctx, cmdctx, fmt.Sprintf("%s.%s", aid, name), cname, r.root, options...)
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Module").
			NewFunctionBuilder().WithFunc(ffiexec.Exec(func(cmd *exec.Cmd) *exec.Cmd {
			return nil
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiexec.Command").NewFunctionBuilder().WithFunc(
			ffigit.Commitish(dir),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Commitish").
			NewFunctionBuilder().WithFunc(
			ffigit.CloneV1(dir),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Clone").
			NewFunctionBuilder().WithFunc(
			ffigit.CloneV2(r.root, r.runtimedir),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.CloneV2").
			NewFunctionBuilder().WithFunc(ffimetric.Metric).
			Export("github.com/egdaemon/eg/runtime/wasi/runtime/metrics.Record")
	}

	if err = r.perform(ctx, runid, module, runtimeenv); err != nil {
		return err
	}

	// if err = os.WriteFile(runid+".dot", []byte(gg.String()), 0600); err != nil {
	// 	return err
	// }

	return nil
}
