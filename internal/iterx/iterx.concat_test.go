package iterx_test

import (
	"errors"
	"testing"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/stretchr/testify/require"
)

func TestConcat(t *testing.T) {
	t.Run("yields each seq in order", func(t *testing.T) {
		seq := iterx.Concat(iterx.Slice(1, 2), iterx.Slice(3), iterx.Slice(4, 5))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
		}

		require.Equal(t, []int{1, 2, 3, 4, 5}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("empty seq as the zero start", func(t *testing.T) {
		seq := iterx.Concat(iterx.EmptySeq[int](), iterx.Slice(1, 2))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
		}

		require.Equal(t, []int{1, 2}, got)
		require.NoError(t, seq.Err())
	})

	t.Run("nothing to yield", func(t *testing.T) {
		seq := iterx.Concat[int]()

		for range seq.Each(t.Context()) {
			require.Fail(t, "unexpected item")
		}
		require.NoError(t, seq.Err())
	})

	t.Run("stops at the first error", func(t *testing.T) {
		boom := errors.New("boom")
		seq := iterx.Concat(iterx.Slice(1), iterx.Error[int](boom), iterx.Slice(2))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
		}

		require.Equal(t, []int{1}, got)
		require.ErrorIs(t, seq.Err(), boom)
	})

	t.Run("stops when the consumer does", func(t *testing.T) {
		seq := iterx.Concat(iterx.Slice(1, 2), iterx.Slice(3))

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
			break
		}

		require.Equal(t, []int{1}, got)
		require.NoError(t, seq.Err())
	})
}
