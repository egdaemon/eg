package coverage_test

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/stretchr/testify/require"
)

func TestOptionsSeq(t *testing.T) {
	src := func(context.Context) coverage.Seq {
		return func(yield func(*coverage.Report, error) bool) {
			for _, p := range []string{"a", "b", "c"} {
				if !yield(&coverage.Report{Path: p}, nil) {
					return
				}
			}
		}
	}

	t.Run("no transforms yields the source", func(t *testing.T) {
		var paths []string
		for r, err := range coverage.New(src).Seq(t.Context()) {
			require.NoError(t, err)
			paths = append(paths, r.Path)
		}
		require.Equal(t, []string{"a", "b", "c"}, paths)
	})

	t.Run("transforms apply in order", func(t *testing.T) {
		suffix := func(s string) coverage.Transform {
			return func(in coverage.Seq) coverage.Seq {
				return func(yield func(*coverage.Report, error) bool) {
					for r, err := range in {
						if r != nil {
							r.Path += s
						}
						if !yield(r, err) {
							return
						}
					}
				}
			}
		}

		var paths []string
		for r, err := range coverage.New(src).Transforms(suffix("1")).Transforms(suffix("2"), suffix("3")).Seq(t.Context()) {
			require.NoError(t, err)
			paths = append(paths, r.Path)
		}
		require.Equal(t, []string{"a123", "b123", "c123"}, paths)
	})

	t.Run("chaining does not modify the receiver", func(t *testing.T) {
		drop := func(in coverage.Seq) coverage.Seq {
			return func(yield func(*coverage.Report, error) bool) {}
		}

		base := coverage.New(src)
		_ = base.Transforms(drop)

		n := 0
		for range base.Seq(t.Context()) {
			n++
		}
		require.Equal(t, 3, n)
	})

	t.Run("errors pass through", func(t *testing.T) {
		failing := func(context.Context) coverage.Seq {
			return func(yield func(*coverage.Report, error) bool) {
				yield(nil, errors.New("boom"))
			}
		}

		var s iter.Seq2[*coverage.Report, error] = coverage.New(failing).Transforms(func(in coverage.Seq) coverage.Seq { return in }).Seq(t.Context())
		for _, err := range s {
			require.EqualError(t, err, "boom")
		}
	})

	t.Run("source is not called until resolved", func(t *testing.T) {
		called := false
		opts := coverage.New(func(context.Context) coverage.Seq {
			called = true
			return src(t.Context())
		})
		require.False(t, called)

		for range opts.Seq(t.Context()) {
		}
		require.True(t, called)
	})

	t.Run("no source is an error", func(t *testing.T) {
		for _, err := range (coverage.Options{}).Seq(t.Context()) {
			require.EqualError(t, err, "no coverage source")
		}
	})

	t.Run("stopping early does not panic", func(t *testing.T) {
		n := 0
		for range coverage.New(src).Seq(t.Context()) {
			n++
			break
		}
		require.Equal(t, 1, n)
	})
}

func TestTransformMarkGenerated(t *testing.T) {
	src := func(context.Context) coverage.Seq {
		return func(yield func(*coverage.Report, error) bool) {
			for _, p := range []string{"a.go", "b.gen.go", "c.go"} {
				if !yield(&coverage.Report{Path: p}, nil) {
					return
				}
			}
		}
	}

	t.Run("marks reports matching any check", func(t *testing.T) {
		var marked []string
		for r, err := range coverage.New(src).Transforms(coverage.TransformMarkGenerated(
			func(r coverage.Report) bool { return r.Path == "a.go" },
			func(r coverage.Report) bool { return r.Path == "b.gen.go" },
		)).Seq(t.Context()) {
			require.NoError(t, err)
			if r.Generated {
				marked = append(marked, r.Path)
			}
		}
		require.Equal(t, []string{"a.go", "b.gen.go"}, marked)
	})

	t.Run("no checks marks nothing", func(t *testing.T) {
		n := 0
		for r, err := range coverage.New(src).Transforms(coverage.TransformMarkGenerated()).Seq(t.Context()) {
			require.NoError(t, err)
			require.False(t, r.Generated)
			n++
		}
		require.Equal(t, 3, n)
	})

	t.Run("does not modify the input", func(t *testing.T) {
		in := &coverage.Report{Path: "a.go"}
		for range coverage.TransformMarkGenerated(func(coverage.Report) bool { return true })(func(yield func(*coverage.Report, error) bool) {
			yield(in, nil)
		}) {
		}
		require.False(t, in.Generated)
	})
}

func TestTransformPath(t *testing.T) {
	boom := errors.New("boom")
	in := &coverage.Report{Path: "a.go", Fnname: "F"}
	src := func(context.Context) coverage.Seq {
		return func(yield func(*coverage.Report, error) bool) {
			if !yield(in, nil) {
				return
			}
			yield(nil, boom)
		}
	}

	var (
		paths []string
		errs  []error
	)
	for r, err := range coverage.New(src).Transforms(coverage.TransformPath(func(p string) string { return "x/" + p })).Seq(t.Context()) {
		if err != nil {
			errs = append(errs, err)
			continue
		}
		paths = append(paths, r.Path)
		require.Equal(t, "F", r.Fnname)
	}

	require.Equal(t, []string{"x/a.go"}, paths)
	require.Equal(t, []error{boom}, errs)
	require.Equal(t, "a.go", in.Path, "input must not be modified")
}

func TestMarkGeneratedExt(t *testing.T) {
	src := func(context.Context) coverage.Seq {
		return func(yield func(*coverage.Report, error) bool) {
			for _, p := range []string{"a.go", "b.gen.go", "c.pb.go", "d.gen.go.txt"} {
				if !yield(&coverage.Report{Path: p}, nil) {
					return
				}
			}
		}
	}

	var marked []string
	for r, err := range coverage.New(src).MarkGeneratedExt(".gen.go", ".pb.go").Seq(t.Context()) {
		require.NoError(t, err)
		if r.Generated {
			marked = append(marked, r.Path)
		}
	}
	require.Equal(t, []string{"b.gen.go", "c.pb.go"}, marked)
}

func TestOptionsRoot(t *testing.T) {
	src := func(context.Context) coverage.Seq {
		return func(yield func(*coverage.Report, error) bool) {
			for _, p := range []string{"src/a.ts", "/abs/b.ts"} {
				if !yield(&coverage.Report{Path: p}, nil) {
					return
				}
			}
		}
	}

	collect := func(opts coverage.Options) (paths []string) {
		for r, err := range opts.Seq(t.Context()) {
			require.NoError(t, err)
			paths = append(paths, r.Path)
		}
		return paths
	}

	t.Run("joins relative paths and keeps absolute ones", func(t *testing.T) {
		require.Equal(t, []string{"/ws/console/src/a.ts", "/abs/b.ts"}, collect(coverage.New(src).Root("/ws/console")))
	})

	t.Run("no root is a no-op", func(t *testing.T) {
		require.Equal(t, []string{"src/a.ts", "/abs/b.ts"}, collect(coverage.New(src)))
	})

	t.Run("applies before transforms whatever the chaining order", func(t *testing.T) {
		suffix := coverage.TransformPath(func(p string) string { return p + "!" })
		expected := []string{"/ws/console/src/a.ts!", "/abs/b.ts!"}

		require.Equal(t, expected, collect(coverage.New(src).Transforms(suffix).Root("/ws/console")))
		require.Equal(t, expected, collect(coverage.New(src).Root("/ws/console").Transforms(suffix)))
	})
}
