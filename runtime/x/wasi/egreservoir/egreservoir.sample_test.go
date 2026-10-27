package egreservoir_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/runtime/x/wasi/egreservoir"
	"github.com/stretchr/testify/require"
)

func collect[T any](ctx context.Context, seq iterx.Seq[T]) (got []T) {
	for v := range seq.Each(ctx) {
		got = append(got, v)
	}
	return got
}

func TestSample(t *testing.T) {
	t.Run("yields every item when n covers the source", func(t *testing.T) {
		seq := egreservoir.Seeded[int]("seed").Sample(10, iterx.Slice(1, 2, 3))

		require.ElementsMatch(t, []int{1, 2, 3}, collect(t.Context(), seq))
		require.NoError(t, seq.Err())
	})

	t.Run("yields n distinct items from the source", func(t *testing.T) {
		seq := egreservoir.Seeded[int]("seed").Sample(3, iterx.Slice(1, 2, 3, 4, 5, 6, 7, 8, 9, 10))

		got := collect(t.Context(), seq)
		require.Len(t, got, 3)
		require.Len(t, map[int]struct{}{got[0]: {}, got[1]: {}, got[2]: {}}, 3)
		for _, v := range got {
			require.GreaterOrEqual(t, v, 1)
			require.LessOrEqual(t, v, 10)
		}
		require.NoError(t, seq.Err())
	})

	t.Run("nothing to yield", func(t *testing.T) {
		for _, n := range []int{-1, 0} {
			seq := egreservoir.Seeded[int]("seed").Sample(n, iterx.Slice(1, 2, 3))
			require.Empty(t, collect(t.Context(), seq))
			require.NoError(t, seq.Err())
		}

		seq := egreservoir.Seeded[int]("seed").Sample(3, iterx.Slice[int]())
		require.Empty(t, collect(t.Context(), seq))
		require.NoError(t, seq.Err())
	})

	t.Run("propagates the source error", func(t *testing.T) {
		failure := errors.New("boom")
		seq := egreservoir.Seeded[int]("seed").Sample(3, iterx.Error[int](failure))

		require.Empty(t, collect(t.Context(), seq))
		require.ErrorIs(t, seq.Err(), failure)
	})

	t.Run("stops with the context's error when cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		seq := egreservoir.Seeded[int]("seed").Sample(3, iterx.Slice(1, 2, 3))

		var got []int
		for v := range seq.Each(ctx) {
			got = append(got, v)
			cancel()
		}

		require.Len(t, got, 1)
		require.ErrorIs(t, seq.Err(), context.Canceled)
	})

	t.Run("stops when the consumer does", func(t *testing.T) {
		seq := egreservoir.Seeded[int]("seed").Sample(3, iterx.Slice(1, 2, 3))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
			break
		}

		require.Len(t, got, 1)
		require.NoError(t, seq.Err())
	})

	t.Run("same seed yields the same sample", func(t *testing.T) {
		items := make([]int, 100)
		for i := range items {
			items[i] = i
		}

		a := collect(t.Context(), egreservoir.Seeded[int]("seed").Sample(5, iterx.Slice(items...)))
		b := collect(t.Context(), egreservoir.Seeded[int]("seed").Sample(5, iterx.Slice(items...)))
		c := collect(t.Context(), egreservoir.Seeded[int]("other").Sample(5, iterx.Slice(items...)))

		require.Equal(t, a, b)
		require.NotEqual(t, a, c)
	})

	t.Run("selection is roughly uniform", func(t *testing.T) {
		const runs = 4000
		counts := map[int]int{}
		for i := range runs {
			for _, v := range collect(t.Context(), egreservoir.Seeded[int](strconv.Itoa(i)).Sample(1, iterx.Slice(0, 1, 2, 3))) {
				counts[v]++
			}
		}

		for v := range 4 {
			require.InDelta(t, runs/4, counts[v], 200, "item %d", v)
		}
	})
}
