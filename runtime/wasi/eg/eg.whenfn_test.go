package eg_test

import (
	"context"
	"errors"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/stretchr/testify/require"
)

func TestWhenFn(t *testing.T) {
	t.Run("executes when the function returns true", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		c := egtest.NewBuffer()
		require.NoError(t, eg.Perform(ctx, eg.WhenFn(func(context.Context) bool { return true }, c.Op('a'))))
		require.Equal(t, []byte{'a'}, c.Current())
	})

	t.Run("skips when the function returns false", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		c := egtest.NewBuffer()
		require.NoError(t, eg.Perform(ctx, eg.WhenFn(func(context.Context) bool { return false }, c.Op('a'))))
		require.Empty(t, c.Current())
	})

	t.Run("propagates the operation error", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		cause := errors.New("boom")
		err := eg.Perform(ctx, eg.WhenFn(func(context.Context) bool { return true }, func(context.Context, eg.Op) error { return cause }))
		require.ErrorIs(t, err, cause)
	})
}
