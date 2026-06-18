// Package egautogentest drives language-specific test generators against
// functions selected from the coverage data eg already records while
// running a module's tests.
package egautogentest

import (
	"context"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoverage"
)

// Fn identifies a single function discovered from recorded coverage data.
type Fn struct {
	Path string
	Name string
	Hits int
}

// Generator implements language specific test generation for a sequence of
// functions.
type Generator interface {
	Generate(iterx.Seq[Fn]) eg.OpFn
}

// Worst scans recorded coverage for the n functions with the lowest hit counts.
func Worst(n int) iterx.Seq[Fn] {
	return iterx.New(func(ctx context.Context, yield func(Fn) bool) error {
		results, err := fficoverage.Worst(ctx, int32(n))
		if err != nil {
			return err
		}

		for _, c := range results {
			if !yield(Fn{Path: c.Path, Name: c.Fnname, Hits: int(c.Hits)}) {
				return nil
			}
		}

		return nil
	})
}

// From builds a Seq directly from a fixed set of functions, useful for
// driving a Generator without recorded coverage data (e.g. in tests or
// one-off manual runs).
func From(fns ...Fn) iterx.Seq[Fn] {
	return iterx.New(func(ctx context.Context, yield func(Fn) bool) error {
		for _, fn := range fns {
			if !yield(fn) {
				return nil
			}
		}

		return nil
	})
}

// Sample returns a random sample of n functions from recorded coverage data.
func Sample(n int) iterx.Seq[Fn] {
	return iterx.New(func(ctx context.Context, yield func(Fn) bool) error {
		results, err := fficoverage.Sample(ctx, int32(n))
		if err != nil {
			return err
		}

		for _, c := range results {
			if !yield(Fn{Path: c.Path, Name: c.Fnname, Hits: int(c.Hits)}) {
				return nil
			}
		}

		return nil
	})
}
