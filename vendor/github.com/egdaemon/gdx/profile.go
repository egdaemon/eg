package gdx

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"runtime/pprof"
	"runtime/trace"

	"github.com/egdaemon/gdx/internal/errorsx"
)

// Profile dispatches to CPU/Heap/Allocs/Block by mode, returning a reader whose
// Read will fail with an "unknown profile mode" error for any other mode.
func Profile(ctx context.Context, mode ProfileMode) io.Reader {
	switch mode {
	case ProfileMode_cpu:
		return CPU(ctx)
	case ProfileMode_heap, ProfileMode_mem:
		return Heap(ctx)
	case ProfileMode_allocs:
		return Allocs(ctx)
	case ProfileMode_block:
		return Block(ctx)
	default:
		return pipe(func(w io.Writer) error {
			return fmt.Errorf("unknown profile mode: %s", mode)
		})
	}
}

// CPU returns a reader streaming a CPU profile for the duration of ctx. it does
// not touch runtime/trace, unlike the genieql-lineage debugx.Profile which
// started a trace unconditionally regardless of mode.
func CPU(ctx context.Context) io.Reader {
	return pipe(func(w io.Writer) error {
		if err := pprof.StartCPUProfile(w); err != nil {
			return fmt.Errorf("unable to start cpu profile: %w", err)
		}

		<-ctx.Done()
		pprof.StopCPUProfile()

		return errorsx.Ignore(ctx.Err(), context.DeadlineExceeded)
	})
}

// Memory returns a reader streaming a heap profile snapshot once ctx is done.
func Memory(ctx context.Context) io.Reader {
	return pipe(func(w io.Writer) error {
		<-ctx.Done()
		return pprof.Lookup("heap").WriteTo(w, 0)
	})
}

// Heap returns a reader streaming a heap profile snapshot once ctx is done.
func Heap(ctx context.Context) io.Reader {
	return pipe(func(w io.Writer) error {
		<-ctx.Done()
		return pprof.Lookup("heap").WriteTo(w, 0)
	})
}

// Allocs returns a reader streaming an allocation profile snapshot once ctx is done.
func Allocs(ctx context.Context) io.Reader {
	return pipe(func(w io.Writer) error {
		<-ctx.Done()
		return pprof.Lookup("allocs").WriteTo(w, 0)
	})
}

// Block returns a reader streaming a blocking profile for the duration of ctx.
func Block(ctx context.Context) io.Reader {
	return pipe(func(w io.Writer) error {
		runtime.SetBlockProfileRate(1)
		defer runtime.SetBlockProfileRate(0)

		<-ctx.Done()

		if err := pprof.Lookup("block").WriteTo(w, 0); err != nil {
			return err
		}

		return errorsx.Ignore(ctx.Err(), context.DeadlineExceeded)
	})
}

// Trace returns a reader streaming a runtime/trace execution trace for the duration of ctx.
func Trace(ctx context.Context) io.Reader {
	return pipe(func(w io.Writer) error {
		if err := trace.Start(w); err != nil {
			return fmt.Errorf("unable to start trace: %w", err)
		}

		<-ctx.Done()
		trace.Stop()

		return errorsx.Ignore(ctx.Err(), context.DeadlineExceeded)
	})
}

// pipe runs capture in a goroutine, streaming whatever it writes through the
// returned reader, and closes the pipe with capture's error (if any) once it
// returns.
func pipe(capture func(w io.Writer) error) io.Reader {
	r, w := io.Pipe()

	go func() {
		w.CloseWithError(capture(w))
	}()

	return r
}
