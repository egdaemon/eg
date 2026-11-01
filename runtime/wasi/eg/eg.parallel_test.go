package eg_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/stretchr/testify/require"
)

func TestParallel(t *testing.T) {
	t.Run("executes every operation", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		c := egtest.NewBuffer()
		require.NoError(t, eg.Perform(ctx, eg.Parallel(c.Op('a'), c.Op('b'), c.Op('c'))))
		res := c.Current()
		slices.Sort(res)
		require.Equal(t, []byte{'a', 'b', 'c'}, res)
	})

	t.Run("no operations is a noop", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		require.NoError(t, eg.Perform(ctx, eg.Parallel()))
	})

	t.Run("reports errors and still runs the other operations", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		c := egtest.NewBuffer()
		cause := errors.New("boom")
		err := eg.Perform(ctx, eg.Parallel(c.Op('a'), func(context.Context, eg.Op) error { return cause }, c.Op('b')))
		require.ErrorIs(t, err, cause)
		res := c.Current()
		slices.Sort(res)
		require.Equal(t, []byte{'a', 'b'}, res)
	})
}
