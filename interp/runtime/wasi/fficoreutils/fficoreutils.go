// Package fficoreutils is the host side of the coreutils binding. It runs utilities from a
// coreutils reactor module (exports uu_alloc, uu_free, uu_chdir and uu_<name>) on behalf of guests.
package fficoreutils

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"sync"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy/v2/proc"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"
)

const (
	exitHostFailure = 127
)

// Module is the file name of the coreutils reactor in the guest runtime's module directory.
const Module = "coreutils.wasi"

// Streams are the stdio of a single call. nil members fall back to the defaults the reactor was
// created with.
type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type streamsKey struct{}

// WithStreams attaches the stdio to use for calls made with the returned context. embedders run a
// guest with a context carrying that guest's stdio, so the calls it makes through the host
// function read and write it directly.
func WithStreams(ctx context.Context, s Streams) context.Context {
	return context.WithValue(ctx, streamsKey{}, s)
}

// switchable stdio, and the done channel of the call using them. an instance is created once with these, and
// they are only changed by the call that holds the instance, which is also the only time the instance can be
// using them.
type stdio struct {
	in   io.Reader
	out  io.Writer
	err  io.Writer
	done <-chan struct{}
}

// sleep pauses for ns, or until the call holding the instance is done. a call blocked in a host function, as a
// sleep is, can't be stopped by closing its module, so this is what ends a sleeping utility at its deadline.
func (t *stdio) sleep(ns int64) {
	timer := time.NewTimer(time.Duration(ns))
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-t.done:
	}
}

type stdinSwitch struct{ s *stdio }

func (t stdinSwitch) Read(p []byte) (int, error) {
	if t.s.in == nil {
		return 0, io.EOF
	}
	return t.s.in.Read(p)
}

type stdoutSwitch struct{ s *stdio }

func (t stdoutSwitch) Write(p []byte) (int, error) {
	if t.s.out == nil {
		return len(p), nil
	}
	return t.s.out.Write(p)
}

type stderrSwitch struct{ s *stdio }

func (t stderrSwitch) Write(p []byte) (int, error) {
	if t.s.err == nil {
		return len(p), nil
	}
	return t.s.err.Write(p)
}

// Reactor runs utilities on instances of the coreutils module. an instance runs one call at a time, so a call
// takes an idle instance, or instantiates one when every instance is busy, and returns it when it finishes. calls
// run concurrently, as the utilities of a pipeline must. an instance a call fails on (process::exit, a trap, the
// call's deadline) is closed instead of returned.
type Reactor struct {
	rt       wazero.Runtime
	compiled wazero.CompiledModule
	loaded   bool
	mcfg     wazero.ModuleConfig

	defaults Streams

	mu     sync.Mutex
	idle   []*instance
	closed bool
	exits  sync.Map
}

// instance of the module, with the stdio its calls switch to their own.
type instance struct {
	mod   api.Module
	stdio *stdio
}

// New creates a reactor. the wasi_snapshot_preview1 module must already be instantiated in the
// runtime, and the runtime must close modules when their context is done for a call's deadline to stop it.
// defaults is the stdio used by calls that do not attach their own with WithStreams. mcfg provides everything
// else (mounts, environment, ...).
func New(rt wazero.Runtime, compiled wazero.CompiledModule, mcfg wazero.ModuleConfig, defaults Streams) *Reactor {
	return &Reactor{
		rt:       rt,
		compiled: compiled,
		defaults: defaults,
		// anonymous, so instances do not collide. the reactor has no _start, and _initialize runs the module's
		// initialization when it has one.
		mcfg: mcfg.WithName("").WithStartFunctions("_initialize").WithArgs("coreutils"),
	}
}

// Load creates a reactor for the module at path. without the file the reactor provides no utilities.
func Load(ctx context.Context, rt wazero.Runtime, path string, mcfg wazero.ModuleConfig, defaults Streams) (_ *Reactor, err error) {
	var (
		bin      []byte
		compiled wazero.CompiledModule
		r        *Reactor
	)

	if bin, err = os.ReadFile(path); errors.Is(err, fs.ErrNotExist) {
		return New(rt, nil, mcfg, defaults), nil
	} else if err != nil {
		return nil, errorsx.Wrapf(err, "unable to read coreutils %s", path)
	}

	if compiled, err = rt.CompileModule(ctx, bin); err != nil {
		return nil, errorsx.Wrapf(err, "unable to compile coreutils %s", path)
	}

	r = New(rt, compiled, mcfg, defaults)
	r.loaded = true
	return r, nil
}

// Export adds the reactor's host functions to the guest's host module. runtimedir is the host path of the guest's
// runtime directory, where the guest listens on the stdio of the utilities it starts.
func (t *Reactor) Export(host wazero.HostModuleBuilder, runtimedir string) wazero.HostModuleBuilder {
	return host.
		NewFunctionBuilder().WithFunc(t.Run()).Export("github.com/egdaemon/eg/runtime/wasi/runtime/fficoreutils.Run").
		NewFunctionBuilder().WithFunc(t.Start(runtimedir)).Export("github.com/egdaemon/eg/runtime/wasi/runtime/fficoreutils.Start").
		NewFunctionBuilder().WithFunc(t.Wait()).Export("github.com/egdaemon/eg/runtime/wasi/runtime/fficoreutils.Wait")
}

// Close releases the idle instances, and the module when Load compiled it. instances still running a call are
// closed when it finishes.
func (t *Reactor) Close(ctx context.Context) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true
	for _, inst := range t.idle {
		err = errors.Join(err, inst.mod.Close(ctx))
	}
	t.idle = nil

	if t.loaded {
		err = errors.Join(err, t.compiled.Close(ctx))
	}

	return err
}

// acquire an idle instance, or instantiate one when there is none.
func (t *Reactor) acquire(ctx context.Context) (_ *instance, err error) {
	var (
		inst *instance
		sio  = &stdio{}
		mod  api.Module
	)

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, errorsx.String("coreutils is closed")
	}

	if n := len(t.idle); n > 0 {
		inst, t.idle = t.idle[n-1], t.idle[:n-1]
		t.mu.Unlock()
		return inst, nil
	}
	t.mu.Unlock()

	if mod, err = t.rt.InstantiateModule(ctx, t.compiled, t.mcfg.WithStdin(stdinSwitch{sio}).WithStdout(stdoutSwitch{sio}).WithStderr(stderrSwitch{sio}).WithNanosleep(sio.sleep)); err != nil {
		return nil, errorsx.Wrap(err, "unable to instantiate coreutils")
	}

	return &instance{mod: mod, stdio: sio}, nil
}

// release the instance to the idle ones, or close it once the reactor is closed.
func (t *Reactor) release(ctx context.Context, inst *instance) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		errorsx.Log(inst.mod.Close(ctx))
		return
	}

	t.idle = append(t.idle, inst)
}

// Call runs the utility in dir, a path within the module's mounts or / when empty, and returns its exit code.
func (t *Reactor) Call(ctx context.Context, dir string, name string, args []string) (code uint32, err error) {
	var (
		inst    *instance
		streams Streams
	)

	if t.compiled == nil {
		return 0, errorsx.Errorf("coreutils is not installed, unable to run %s", name)
	}

	if inst, err = t.acquire(ctx); err != nil {
		return 0, err
	}

	// stdio is only used while a call runs, and the instance runs only this one.
	streams, _ = ctx.Value(streamsKey{}).(Streams)
	inst.stdio.in = cmp.Or(streams.Stdin, t.defaults.Stdin)
	inst.stdio.out = cmp.Or(streams.Stdout, t.defaults.Stdout)
	inst.stdio.err = cmp.Or(streams.Stderr, t.defaults.Stderr)
	inst.stdio.done = ctx.Done()

	code, err = t.invoke(ctx, inst.mod, cmp.Or(dir, "/"), name, args)
	*inst.stdio = stdio{}

	if err != nil {
		// the instance may be in an undefined state, never reuse it after a failure.
		errorsx.Log(inst.mod.Close(ctx))
		return code, err
	}

	t.release(ctx, inst)
	return code, nil
}

func (t *Reactor) invoke(ctx context.Context, inst api.Module, dir string, name string, args []string) (_ uint32, err error) {
	var (
		argv  bytes.Buffer
		res   []uint64
		exit  *sys.ExitError
		fn    = inst.ExportedFunction("uu_" + name)
		chdir = inst.ExportedFunction("uu_chdir")
	)

	if fn == nil || chdir == nil || inst.ExportedFunction("uu_alloc") == nil || inst.ExportedFunction("uu_free") == nil {
		return 0, errorsx.Errorf("coreutils does not provide %q", name)
	}

	// the instance is shared by calls from different directories, and keeps the last one it changed to.
	if res, err = t.with(ctx, inst, []byte(dir), func(ptr uint64, n uint64) ([]uint64, error) {
		return chdir.Call(ctx, ptr, n)
	}); err != nil {
		return 0, errorsx.Wrapf(err, "unable to change to %s", dir)
	}

	if errno := int32(res[0]); errno != 0 {
		return 0, errorsx.Errorf("unable to change to %s: errno %d", dir, errno)
	}

	// argv, each entry (argv[0] included) terminated by NUL.
	for _, a := range append([]string{name}, args...) {
		argv.WriteString(a)
		argv.WriteByte(0)
	}

	if res, err = t.with(ctx, inst, argv.Bytes(), func(ptr uint64, n uint64) ([]uint64, error) {
		return fn.Call(ctx, ptr, n)
	}); err != nil {
		// utilities may call process::exit, which surfaces as an exit error. the instance is
		// closed by then, which Call handles.
		if errors.As(err, &exit) {
			return exit.ExitCode(), errorsx.Wrapf(err, "%s exited the instance", name)
		}
		return 0, errorsx.Wrapf(err, "%s failed", name)
	}

	return uint32(int32(res[0])), nil
}

// with copies data into memory the instance allocates for it, calls do with its address and length, and releases
// the memory once do returns.
func (t *Reactor) with(ctx context.Context, inst api.Module, data []byte, do func(ptr uint64, n uint64) ([]uint64, error)) (res []uint64, err error) {
	var (
		ptr uint64
		n   = uint64(len(data))
	)

	if res, err = inst.ExportedFunction("uu_alloc").Call(ctx, n); err != nil {
		return nil, errorsx.Wrap(err, "unable to allocate")
	}
	ptr = res[0]

	if !inst.Memory().Write(uint32(ptr), data) {
		return nil, errorsx.String("unable to write to the allocation")
	}

	if res, err = do(ptr, n); err != nil {
		return nil, err
	}

	if _, err = inst.ExportedFunction("uu_free").Call(ctx, ptr, n); err != nil {
		return nil, errorsx.Wrap(err, "unable to release the allocation")
	}

	return res, nil
}

// Run returns the host function guests call to run a utility. it returns the exit code.
//
// the utility reads and writes the stdio carried by the context of the call (WithStreams), which
// the embedder attaches to the stdio of the calling guest: output streams as it is produced and
// stdout/stderr keep their order. without streams the reactor defaults apply.
func (t *Reactor) Run() func(
	ctx context.Context,
	m api.Module,
	deadline int64,
	nameoffset uint32, namelen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64,
		nameoffset uint32, namelen uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		ictx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		name, err := ffi.ReadString(m.Memory(), nameoffset, namelen)
		if err != nil {
			log.Println("unable to read coreutils name", err)
			return exitHostFailure
		}

		args, err := ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize)
		if err != nil {
			log.Println("unable to read coreutils arguments", err)
			return exitHostFailure
		}

		code, err := t.Call(ictx, "", name, args)
		if err != nil {
			log.Println("coreutils failed", name, err)
			if code == 0 {
				code = exitHostFailure
			}
		}

		return code
	}
}

// Start returns the host function guests call to start a utility in dir, with its stdio on the guest's sockets in
// proc/{id} within runtimedir. it returns at once and the utility runs in the background: the guest is a single
// thread, and copies the utility's stdio while it runs. it returns ffierrors.ErrNotFound for a utility the module
// does not provide.
func (t *Reactor) Start(runtimedir string) func(
	ctx context.Context,
	m api.Module,
	deadline int64,
	idoffset uint32, idlen uint32,
	diroffset uint32, dirlen uint32,
	nameoffset uint32, namelen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64,
		idoffset uint32, idlen uint32,
		diroffset uint32, dirlen uint32,
		nameoffset uint32, namelen uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		var (
			err   error
			id    string
			dir   string
			name  string
			args  []string
			files []*os.File
			ictx  context.Context
			done  context.CancelFunc
			exit  = make(chan uint32, 1)
		)

		if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
			log.Println("unable to read coreutils name", err)
			return ffierrors.ErrUnrecoverable
		}

		if t.compiled == nil || t.compiled.ExportedFunctions()["uu_"+name] == nil {
			return ffierrors.ErrNotFound
		}

		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read coreutils process id", err)
			return ffierrors.ErrUnrecoverable
		}

		if dir, err = ffi.ReadString(m.Memory(), diroffset, dirlen); err != nil {
			log.Println("unable to read coreutils directory", err)
			return ffierrors.ErrUnrecoverable
		}

		if args, err = ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize); err != nil {
			log.Println("unable to read coreutils arguments", err)
			return ffierrors.ErrUnrecoverable
		}

		if files, err = proc.Dial(runtimedir, id); err != nil {
			log.Println(err)
			return ffierrors.ErrUnrecoverable
		}

		t.exits.Store(id, exit)

		ictx, done = ffi.ReadMicroDeadline(ctx, deadline)
		go func() {
			var (
				code     uint32
				cause    error
				closeall = func() {
					for _, f := range files {
						f.Close()
					}
				}
			)

			defer done()

			// closing the sockets at the deadline ends a utility blocked reading or writing them, which closing its
			// module can't.
			defer context.AfterFunc(ictx, closeall)()
			defer closeall()

			code, cause = t.Call(WithStreams(ictx, Streams{Stdin: files[0], Stdout: files[1], Stderr: files[2]}), dir, name, args)
			if cause != nil {
				log.Println("coreutils failed", name, cause)
				code = cmp.Or(code, exitHostFailure)
			}

			exit <- code
		}()

		return 0
	}
}

// Wait returns the host function guests call for the exit code of the utility started as the process id. it
// returns ffierrors.ErrPending while the utility runs.
func (t *Reactor) Wait() func(ctx context.Context, m api.Module, idoffset uint32, idlen uint32) uint32 {
	return func(ctx context.Context, m api.Module, idoffset uint32, idlen uint32) uint32 {
		var (
			err  error
			id   string
			exit any
			ok   bool
		)

		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read coreutils process id", err)
			return ffierrors.ErrUnrecoverable
		}

		if exit, ok = t.exits.Load(id); !ok {
			return ffierrors.ErrNotFound
		}

		select {
		case code := <-exit.(chan uint32):
			t.exits.Delete(id)
			return code
		default:
			return ffierrors.ErrPending
		}
	}
}
