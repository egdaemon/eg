// Package fficoreutils is the host side of the coreutils binding. It runs utilities from a
// coreutils reactor module (exports uu_alloc, uu_free and uu_<name>) on behalf of guests.
package fficoreutils

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"io"
	"log"
	"sync"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"
)

const (
	exitHostFailure = 127
)

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

// switchable stdio. the instance is created once with these, they are only ever changed while
// the reactor's lock is held, which is also the only time the instance can be using them.
type stdio struct {
	in  io.Reader
	out io.Writer
	err io.Writer
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

// Reactor keeps a single long lived instance of the coreutils module. wasm instances are single
// threaded, so calls from many goroutines are serialized onto it. if a utility kills the instance
// (process::exit, a trap) the next call transparently starts a new one.
type Reactor struct {
	rt       wazero.Runtime
	compiled wazero.CompiledModule
	mcfg     wazero.ModuleConfig

	defaults Streams

	mu    sync.Mutex
	inst  api.Module
	stdio *stdio
}

// New creates a reactor. the wasi_snapshot_preview1 module must already be instantiated in the
// runtime. defaults is the stdio used by calls that do not attach their own with WithStreams.
// calls are serialized so whole outputs never interleave. mcfg provides everything else (mounts,
// environment, ...).
func New(rt wazero.Runtime, compiled wazero.CompiledModule, mcfg wazero.ModuleConfig, defaults Streams) *Reactor {
	sio := &stdio{}
	return &Reactor{
		rt:       rt,
		compiled: compiled,
		defaults: defaults,
		stdio:    sio,
		// anonymous, no start function: the reactor has neither _start nor _initialize.
		mcfg: mcfg.WithName("").WithStartFunctions().WithArgs("coreutils").
			WithStdin(stdinSwitch{sio}).WithStdout(stdoutSwitch{sio}).WithStderr(stderrSwitch{sio}),
	}
}

// Close releases the instance.
func (t *Reactor) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.drop(ctx)
}

func (t *Reactor) drop(ctx context.Context) (err error) {
	if t.inst == nil {
		return nil
	}

	err = t.inst.Close(ctx)
	t.inst = nil
	return err
}

// Call runs the utility and returns its exit code.
func (t *Reactor) Call(ctx context.Context, name string, args []string) (uint32, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// stdio is only used while a call runs, which is while the lock is held.
	streams, _ := ctx.Value(streamsKey{}).(Streams)
	t.stdio.in = cmp.Or(streams.Stdin, t.defaults.Stdin)
	t.stdio.out = cmp.Or(streams.Stdout, t.defaults.Stdout)
	t.stdio.err = cmp.Or(streams.Stderr, t.defaults.Stderr)
	defer func() { *t.stdio = stdio{} }()

	if t.inst == nil {
		inst, err := t.rt.InstantiateModule(ctx, t.compiled, t.mcfg)
		if err != nil {
			return 0, errorsx.Wrap(err, "unable to instantiate coreutils")
		}
		t.inst = inst
	}

	code, err := t.invoke(ctx, t.inst, name, args)
	if err != nil {
		// the instance may be in an undefined state, never reuse it after a failure.
		_ = t.drop(ctx)
		return code, err
	}

	return code, nil
}

func (t *Reactor) invoke(ctx context.Context, inst api.Module, name string, args []string) (uint32, error) {
	fn := inst.ExportedFunction("uu_" + name)
	alloc := inst.ExportedFunction("uu_alloc")
	free := inst.ExportedFunction("uu_free")
	if fn == nil || alloc == nil || free == nil {
		return 0, errorsx.Errorf("coreutils does not provide %q", name)
	}

	// argv, each entry (argv[0] included) terminated by NUL.
	var argv bytes.Buffer
	for _, a := range append([]string{name}, args...) {
		argv.WriteString(a)
		argv.WriteByte(0)
	}

	res, err := alloc.Call(ctx, uint64(argv.Len()))
	if err != nil {
		return 0, errorsx.Wrap(err, "unable to allocate arguments")
	}
	ptr := uint32(res[0])

	if !inst.Memory().Write(ptr, argv.Bytes()) {
		return 0, errors.New("unable to write arguments")
	}

	res, err = fn.Call(ctx, uint64(ptr), uint64(argv.Len()))
	if err != nil {
		// utilities may call process::exit, which surfaces as an exit error. the instance is
		// closed by then, which Call handles.
		var exit *sys.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), errorsx.Wrapf(err, "%s exited the instance", name)
		}
		return 0, errorsx.Wrapf(err, "%s failed", name)
	}

	if _, err = free.Call(ctx, uint64(ptr), uint64(argv.Len())); err != nil {
		return 0, errorsx.Wrap(err, "unable to release arguments")
	}

	return uint32(int32(res[0])), nil
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

		code, err := t.Call(ictx, name, args)
		if err != nil {
			log.Println("coreutils failed", name, err)
			if code == 0 {
				code = exitHostFailure
			}
		}

		return code
	}
}
