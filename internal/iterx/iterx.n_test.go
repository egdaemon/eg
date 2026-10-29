package iterx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/stretchr/testify/require"
)

func TestN(t *testing.T) {
	t.Run("yields at most n items in order", func(t *testing.T) {
		seq := iterx.N(2, iterx.Slice(1, 2, 3))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
		}

		require.Equal(t, []int{1, 2}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("0 yields every item", func(t *testing.T) {
		seq := iterx.N(0, iterx.Slice(1, 2, 3))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
		}

		require.Equal(t, []int{1, 2, 3}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("more than the items yields every item", func(t *testing.T) {
		seq := iterx.N(5, iterx.Slice(1, 2))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
		}

		require.Equal(t, []int{1, 2}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("does not read another item once n have been yielded", func(t *testing.T) {
		var read int
		seq := iterx.N(2, iterx.New(func(ctx context.Context, yield func(int) bool) error {
			for i := range 5 {
				read++
				if !yield(i) {
					return nil
				}
			}

			return nil
		}))

		for range seq.Each(t.Context()) {
		}

		require.Equal(t, 2, read)
		require.NoError(t, seq.Err())
	})

	t.Run("stops when the consumer does", func(t *testing.T) {
		seq := iterx.N(3, iterx.Slice(1, 2, 3))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
			break
		}

		require.Equal(t, []int{1}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("the error of the seq is the error of the result", func(t *testing.T) {
		cause := errors.New("failing")
		seq := iterx.N(2, iterx.Error[int](cause))

		for range seq.Each(t.Context()) {
			require.Fail(t, "unexpected item")
		}

		require.ErrorIs(t, seq.Err(), cause)
	})
}
