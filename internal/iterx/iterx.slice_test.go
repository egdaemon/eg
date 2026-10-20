package iterx_test

import (
	"context"
	"testing"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/stretchr/testify/require"
)

func TestSlice(t *testing.T) {
	t.Run("yields every item in order", func(t *testing.T) {
		seq := iterx.Slice(1, 2, 3)

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
		}

		require.Equal(t, []int{1, 2, 3}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("nothing to yield", func(t *testing.T) {
		seq := iterx.Slice[int]()

		for range seq.Each(t.Context()) {
			require.Fail(t, "unexpected item")
		}
		require.NoError(t, seq.Err())
	})

	t.Run("stops when the consumer does", func(t *testing.T) {
		seq := iterx.Slice(1, 2, 3)

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
			break
		}

		require.Equal(t, []int{1}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("stops with the context's error when cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		seq := iterx.Slice(1, 2, 3)

		var got []int
		for v := range seq.Each(ctx) {
			got = append(got, v)
			cancel()
		}

		require.Equal(t, []int{1}, got)
		require.ErrorIs(t, seq.Err(), context.Canceled)
	})
}
