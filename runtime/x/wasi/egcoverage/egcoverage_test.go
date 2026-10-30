package egcoverage_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/runtime/x/wasi/egcoverage"
	"github.com/stretchr/testify/require"
)

func TestInto(t *testing.T) {
	source := func(n int) coverage.Options {
		return coverage.New(func(context.Context) coverage.Seq {
			return func(yield func(*coverage.Report, error) bool) {
				for range n {
					if !yield(&coverage.Report{}, nil) {
						return
					}
				}
			}
		})
	}

	t.Run("flushes full batches then the remainder", func(t *testing.T) {
		var sizes []int
		sink := func(_ context.Context, batch ...*coverage.Report) error {
			sizes = append(sizes, len(batch))
			return nil
		}

		require.NoError(t, egcoverage.Into(sink, source(130)...)(t.Context(), nil))
		require.Equal(t, []int{128, 2}, sizes)
	})

	t.Run("an empty source flushes an empty batch", func(t *testing.T) {
		var sizes []int
		sink := func(_ context.Context, batch ...*coverage.Report) error {
			sizes = append(sizes, len(batch))
			return nil
		}

		require.NoError(t, egcoverage.Into(sink, source(0)...)(t.Context(), nil))
		require.Equal(t, []int{0}, sizes)
	})

	t.Run("transforms are applied", func(t *testing.T) {
		var got []*coverage.Report
		sink := func(_ context.Context, batch ...*coverage.Report) error {
			got = slices.Concat(got, batch)
			return nil
		}

		opts := source(2).Transforms(coverage.TransformMarkGenerated(func(coverage.Report) bool { return true }))
		require.NoError(t, egcoverage.Into(sink, opts...)(t.Context(), nil))
		require.Len(t, got, 2)
		require.True(t, got[0].Generated)
		require.True(t, got[1].Generated)
	})

	t.Run("source errors stop the operation", func(t *testing.T) {
		boom := errors.New("boom")
		opts := coverage.New(func(context.Context) coverage.Seq {
			return func(yield func(*coverage.Report, error) bool) {
				yield(nil, boom)
			}
		})

		called := false
		sink := func(context.Context, ...*coverage.Report) error {
			called = true
			return nil
		}

		require.ErrorIs(t, egcoverage.Into(sink, opts...)(t.Context(), nil), boom)
		require.False(t, called)
	})

	t.Run("sink errors stop the operation", func(t *testing.T) {
		boom := errors.New("boom")
		sink := func(context.Context, ...*coverage.Report) error { return boom }

		require.ErrorIs(t, egcoverage.Into(sink, source(1)...)(t.Context(), nil), boom)
		require.ErrorIs(t, egcoverage.Into(sink, source(130)...)(t.Context(), nil), boom)
	})

	t.Run("no options is an error", func(t *testing.T) {
		sink := func(context.Context, ...*coverage.Report) error { return nil }
		require.ErrorContains(t, egcoverage.Into(sink)(t.Context(), nil), "no coverage source")
	})
}
