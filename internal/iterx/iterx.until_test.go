package iterx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/stretchr/testify/require"
)

func TestUntil(t *testing.T) {
	t.Run("stops once less than the grace remains before the deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 550*time.Millisecond)
		defer cancel()
		seq := iterx.Until(200*time.Millisecond, iterx.Slice(1, 2, 3, 4, 5, 6, 7, 8, 9, 10))

		var got []int
		for v := range seq.Each(ctx) {
			got = append(got, v)
			time.Sleep(100 * time.Millisecond)
		}

		require.Equal(t, []int{1, 2, 3, 4}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("a context without a deadline yields every item", func(t *testing.T) {
		seq := iterx.Until(time.Minute, iterx.Slice(1, 2, 3))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
		}

		require.Equal(t, []int{1, 2, 3}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("stops when the consumer does", func(t *testing.T) {
		seq := iterx.Until(time.Minute, iterx.Slice(1, 2, 3))

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
		seq := iterx.Until(time.Minute, iterx.Error[int](cause))

		for range seq.Each(t.Context()) {
			require.Fail(t, "unexpected item")
		}

		require.ErrorIs(t, seq.Err(), cause)
	})
}
