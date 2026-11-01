package eg_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/stretchr/testify/require"
)

func TestWhen(t *testing.T) {
	t.Run("executes when true", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		c := egtest.NewBuffer()
		require.NoError(t, eg.Perform(ctx, eg.When(true, c.Op('a'))))
		require.Equal(t, []byte{'a'}, c.Current())
	})

	t.Run("skips when false", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		c := egtest.NewBuffer()
		require.NoError(t, eg.Perform(ctx, eg.When(false, c.Op('a'))))
		require.Empty(t, c.Current())
	})
}
