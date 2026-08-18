package gdx

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"time"

	"github.com/egdaemon/gdx/internal/errorsx"
	"github.com/egdaemon/gdx/internal/langx"
)

// ProfileOption configures profile capture behavior.
type ProfileOption func(*profileConfig)

// profileConfig holds accumulated profile options.
type profileConfig struct {
	duration time.Duration
}

// profileOptions accumulates ProfileOption closures.
type profileOptions []ProfileOption

// ProfileOptions returns a zero-value options slice for chaining.
func ProfileOptions() profileOptions {
	return profileOptions(nil)
}

// Compact returns the options as a plain []ProfileOption slice, suitable for
// variadic function calls.
func (t profileOptions) Compact() []ProfileOption {
	return t
}

// ProfileDuration sets the capture duration. the caller's context may still
// cancel early.
func (t profileOptions) ProfileDuration(d time.Duration) profileOptions {
	return append(t, func(cfg *profileConfig) {
		cfg.duration = d
	})
}

func (t profileOptions) configure(cfg profileConfig) profileConfig {
	for _, opt := range t {
		opt(&cfg)
	}
	return cfg
}

// Profile dispatches to CPU/Heap/Allocs/Block by mode, returning a reader whose
// Read will fail with an "unknown profile mode" error for any other mode.
func Profile(ctx context.Context, mode ProfileMode, opts ...ProfileOption) io.Reader {
	switch mode {
	case ProfileMode_cpu:
		return CPU(ctx, opts...)
	case ProfileMode_heap, ProfileMode_mem:
		return Heap(ctx, opts...)
	case ProfileMode_allocs:
		return Allocs(ctx, opts...)
	case ProfileMode_block:
		return Block(ctx, opts...)
	default:
		return pipe(ctx, func(ctx context.Context, w io.Writer) error {
			return fmt.Errorf("unknown profile mode: %s", mode)
		}, opts...)
	}
}

// CPU returns a reader streaming a CPU profile for the duration of ctx. it does
// not touch runtime/trace, unlike the genieql-lineage debugx.Profile which
// started a trace unconditionally regardless of mode.
func CPU(ctx context.Context, opts ...ProfileOption) io.Reader {
	return pipe(ctx, func(ctx context.Context, w io.Writer) error {
		if err := pprof.StartCPUProfile(w); err != nil {
			return fmt.Errorf("unable to start cpu profile: %w", err)
		}

		<-ctx.Done()
		pprof.StopCPUProfile()

		return nil
	}, opts...)
}

// Memory returns a reader streaming a heap profile snapshot once ctx is done.
func Memory(ctx context.Context, opts ...ProfileOption) io.Reader {
	return pipe(ctx, func(ctx context.Context, w io.Writer) error {
		<-ctx.Done()
		return pprof.Lookup("heap").WriteTo(w, 0)
	}, opts...)
}

// Heap returns a reader streaming a heap profile snapshot once ctx is done.
func Heap(ctx context.Context, opts ...ProfileOption) io.Reader {
	return pipe(ctx, func(ctx context.Context, w io.Writer) error {
		<-ctx.Done()
		return pprof.Lookup("heap").WriteTo(w, 0)
	}, opts...)
}

// Allocs returns a reader streaming an allocation profile snapshot once ctx is done.
func Allocs(ctx context.Context, opts ...ProfileOption) io.Reader {
	return pipe(ctx, func(ctx context.Context, w io.Writer) error {
		<-ctx.Done()
		return pprof.Lookup("allocs").WriteTo(w, 0)
	}, opts...)
}

// Block returns a reader streaming a blocking profile for the duration of ctx.
func Block(ctx context.Context, opts ...ProfileOption) io.Reader {
	return pipe(ctx, func(ctx context.Context, w io.Writer) error {
		runtime.SetBlockProfileRate(1)
		defer runtime.SetBlockProfileRate(0)

		<-ctx.Done()

		if err := pprof.Lookup("block").WriteTo(w, 0); err != nil {
			return err
		}

		return nil
	}, opts...)
}

// Trace returns a reader streaming a runtime/trace execution trace for the duration of ctx.
func Trace(ctx context.Context, opts ...ProfileOption) io.Reader {
	return pipe(ctx, func(ctx context.Context, w io.Writer) error {
		if err := trace.Start(w); err != nil {
			return fmt.Errorf("unable to start trace: %w", err)
		}

		<-ctx.Done()
		trace.Stop()

		return errorsx.Ignore(ctx.Err(), context.DeadlineExceeded)
	}, opts...)
}

// pipe runs capture in a goroutine, streaming whatever it writes through the
// returned reader, and closes the pipe with capture's error (if any) once it
// returns.
func pipe(ctx context.Context, capture func(ctx context.Context, w io.Writer) error, options ...ProfileOption) io.Reader {
	cfg := profileOptions(options).configure(profileConfig{duration: 30 * time.Second})
	r, w := io.Pipe()

	go func() {
		ictx, done := context.WithTimeout(ctx, cfg.duration)
		defer done()
		w.CloseWithError(langx.FirstNonNil(errorsx.Ignore(capture(ictx, w), context.DeadlineExceeded), ctx.Err()))
	}()

	return r
}

// RecordFile captures the reader to disk at path, creating the parent
// directory if needed. the reader is typically the io.Reader returned by
// Profile (e.g. Profile(ctx, ProfileMode_cpu)). it creates a cancellable
// context and cancels it after the copy completes so the capture goroutine
// can exit.
func RecordFile(ctx context.Context, path string, r io.Reader) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("unable to create profile directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create profile file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("unable to record profile: %w", err)
	}

	return nil
}
