package iterx_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/stretchr/testify/require"
)

type closer struct {
	err    error
	closed int
}

func (t *closer) Close() error {
	t.closed++
	return t.err
}

func TestLazy(t *testing.T) {
	t.Run("does not resolve until consumed", func(t *testing.T) {
		var opened int
		seq := iterx.Lazy(func(context.Context) (iterx.Seq[int], io.Closer, error) {
			opened++
			return iterx.Slice(1, 2), nil, nil
		})
		require.Zero(t, opened)

		var got []int
		for v := range seq.Each(t.Context()) {
			got = append(got, v)
		}

		require.Equal(t, []int{1, 2}, got)
		require.NoError(t, seq.Err())
		require.Equal(t, 1, opened)
	})

	t.Run("closes when iteration ends", func(t *testing.T) {
		c := &closer{}
		seq := iterx.Lazy(func(context.Context) (iterx.Seq[int], io.Closer, error) {
			return iterx.Slice(1, 2), c, nil
		})

		for range seq.Each(t.Context()) {
		}

		require.NoError(t, seq.Err())
		require.Equal(t, 1, c.closed)
	})

	t.Run("closes when iteration stops early", func(t *testing.T) {
		c := &closer{}
		seq := iterx.Lazy(func(context.Context) (iterx.Seq[int], io.Closer, error) {
			return iterx.Slice(1, 2), c, nil
		})

		for range seq.Each(t.Context()) {
			break
		}

		require.NoError(t, seq.Err())
		require.Equal(t, 1, c.closed)
	})

	t.Run("joins the close error with the error of the sequence", func(t *testing.T) {
		cause, closeErr := errors.New("cause"), errors.New("close")
		c := &closer{err: closeErr}
		seq := iterx.Lazy(func(context.Context) (iterx.Seq[int], io.Closer, error) {
			return iterx.Error[int](cause), c, nil
		})

		for range seq.Each(t.Context()) {
		}

		require.ErrorIs(t, seq.Err(), cause)
		require.ErrorIs(t, seq.Err(), closeErr)
	})

	t.Run("an error opening is the error of the sequence", func(t *testing.T) {
		cause := errors.New("unavailable")
		seq := iterx.Lazy(func(context.Context) (iterx.Seq[int], io.Closer, error) {
			return nil, nil, cause
		})

		var count int
		for range seq.Each(t.Context()) {
			count++
		}

		require.Zero(t, count)
		require.ErrorIs(t, seq.Err(), cause)
	})
}
