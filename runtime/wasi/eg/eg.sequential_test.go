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

func TestSequential(t *testing.T) {
	t.Run("executes operations in order", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		c := egtest.NewBuffer()
		require.NoError(t, eg.Perform(ctx, eg.Sequential(c.Op('a'), c.Op('b'), c.Op('c'))))
		require.Equal(t, []byte{'a', 'b', 'c'}, c.Current())
	})

	t.Run("no operations is a noop", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		require.NoError(t, eg.Perform(ctx, eg.Sequential()))
	})

	t.Run("stops at the first error", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		c := egtest.NewBuffer()
		cause := errors.New("boom")
		err := eg.Perform(ctx, eg.Sequential(c.Op('a'), func(context.Context, eg.Op) error { return cause }, c.Op('b')))
		require.ErrorIs(t, err, cause)
		require.Equal(t, []byte{'a'}, c.Current())
	})
}
