package main

import (
	"context"
	"crypto/rand"
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/runtime/wasi/fficoreutils"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiwasinet"
	"github.com/egdaemon/eg/interp/runtime/wasi/wasimod"
	"github.com/egdaemon/eg/interp/wasidebug"
	"github.com/egdaemon/eg/workspaces"
	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/gofrs/uuid/v5"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Wasi struct {
	Debug wasiCmdDebug `cmd:"" name:"debug" help:"print the exported and imported functions of a wasi module" hidden:"true"`
	Run   wasiCmdRun   `cmd:"" name:"run" help:"run a standalone wasi module" hidden:"true"`
}

type wasiCmdRun struct {
	Dir    string `name:"directory" help:"root directory of the repository" default:"${vars_git_directory}"`
	Module string `arg:"" help:"name of the module to run"`
}

func (t wasiCmdRun) Run(gctx *cmdopts.Global) (err error) {
	var (
		ws workspaces.Context
	)

	ctx, done := context.WithCancel(gctx.Context)
	defer done()

	if ws, err = workspaces.NewLocal(gctx.Context, uuid.Must(uuid.NewV7()), md5x.Digest(cmdopts.BuildInfoSafe()), t.Dir, t.Module); err != nil {
		return errorsx.Wrap(err, "unable to setup workspace")
	}

	mpath := filepath.Join(ws.RuntimeDir, "test.wasm")
	debugx.Println("wasipath", ws.RuntimeDir, mpath)
	// the module directory is its own go module.
	if err = compile.Run(ctx, eg.DefaultModuleDirectory(t.Dir), t.Module, mpath); err != nil {
		return err
	}
	wruntime := wazero.NewRuntimeWithConfig(
		ctx,
		// closing modules when their context is done is what stops a coreutils call at its deadline.
		wazero.NewRuntimeConfigInterpreter().WithCloseOnContextDone(true),
	)

	// the well known directories, same layout as interp.
	wazerofs := wazero.NewFSConfig().
		WithFSMount(embed.FS{}, "/").
		WithDirMount(os.TempDir(), os.TempDir()).
		WithDirMount(ws.Root, eg.DefaultRootDirectory()).
		WithDirMount(ws.Root, ws.Root).
		WithDirMount(ws.RuntimeDir, eg.DefaultRuntimeDirectory()).
		WithDirMount(ws.RuntimeDir, ws.RuntimeDir).
		WithDirMount(ws.CacheDir, eg.DefaultCacheDirectory()).
		WithDirMount(ws.CacheDir, ws.CacheDir).
		WithDirMount(ws.WorkspaceDir, eg.DefaultWorkspaceDirectory()).
		WithDirMount(ws.WorkspaceDir, ws.WorkspaceDir).
		WithDirMount(ws.WorkingDir, eg.DefaultWorkingDirectory()).
		WithDirMount(ws.WorkingDir, ws.WorkingDir)

	mcfg := wazero.NewModuleConfig().
		// Instructs WASI sched_yield to actually yield the host thread
		WithOsyield(runtime.Gosched).
		// Enables actual sleeping for WASI nanosleep calls
		WithSysNanosleep().
		WithStdin(os.Stdin).
		WithStderr(os.Stderr).
		WithStdout(os.Stdout).
		WithFSConfig(wazerofs).
		WithSysNanotime().
		WithSysWalltime().
		WithRandSource(rand.Reader)

	// the host's PWD is not mounted, the guest's working directory is the workload's as in interp.
	environ := errorsx.Zero(
		envx.Build().
			FromEnviron(os.Environ()...).
			Var("PWD", eg.DefaultWorkingDirectory()).
			Environ(),
	)
	// envx.Debug(environ...)
	mcfg = wasix.Environ(mcfg, environ...)

	wasienv, err := wasi_snapshot_preview1.NewBuilder(wruntime).Instantiate(ctx)
	if err != nil {
		return err
	}
	defer wasienv.Close(ctx)

	// sockets bypass the filesystem mounts, so wasinet remaps their paths itself.
	wasinet, err := ffiwasinet.Wazero(
		wruntime,
		wnetruntime.OptionFSPrefixes(
			wnetruntime.FSPrefix{Host: ws.Root, Guest: eg.DefaultRootDirectory()},
			wnetruntime.FSPrefix{Host: ws.RuntimeDir, Guest: eg.DefaultRuntimeDirectory()},
			wnetruntime.FSPrefix{Host: ws.CacheDir, Guest: eg.DefaultCacheDirectory()},
			wnetruntime.FSPrefix{Host: ws.WorkspaceDir, Guest: eg.DefaultWorkspaceDirectory()},
			wnetruntime.FSPrefix{Host: ws.WorkingDir, Guest: eg.DefaultWorkingDirectory()},
		),
	).Instantiate(ctx)
	if err != nil {
		return err
	}
	defer wasinet.Close(ctx)

	wasimods, err := wasimod.Load(ctx, wruntime, wasimod.Directory(), mcfg)
	if err != nil {
		return errorsx.Wrap(err, "unable to load the wasi modules")
	}
	defer wasimods.Close(ctx)

	coreutils, err := fficoreutils.Unavailable(ctx, wruntime)
	if err != nil {
		return errorsx.Wrap(err, "unable to stand in for the coreutils reactor")
	}
	defer coreutils.Close(ctx)

	log.Println("interp initiated", mpath)
	defer log.Println("interp completed", mpath)
	wasi, err := os.ReadFile(mpath)
	if err != nil {
		return err
	}

	c, err := wruntime.CompileModule(ctx, wasi)
	if err != nil {
		return err
	}
	defer c.Close(ctx)

	// wasidebug.Module(c)

	m, err := wruntime.InstantiateModule(ctx, c, mcfg.WithName(mpath))
	if err != nil {
		return err
	}
	defer m.Close(ctx)

	return nil
}

// wasiCmdDebug compiles a wasi module and prints its exported and imported functions.
type wasiCmdDebug struct {
	Path string `arg:"" help:"path to the wasi module to debug" type:"existingfile"`
}

func (t wasiCmdDebug) Run(gctx *cmdopts.Global) error {
	raw, err := os.ReadFile(t.Path)
	if err != nil {
		return errorsx.Wrap(err, "unable to read module")
	}

	runtime := wazero.NewRuntimeWithConfig(
		gctx.Context,
		wazero.NewRuntimeConfigInterpreter(),
	)
	defer runtime.Close(gctx.Context)

	c, err := runtime.CompileModule(gctx.Context, raw)
	if err != nil {
		return errorsx.Wrap(err, "unable to compile module")
	}
	defer c.Close(gctx.Context)

	wasidebug.Module(c)

	return nil
}
