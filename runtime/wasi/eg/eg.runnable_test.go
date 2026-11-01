package eg_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/stretchr/testify/require"
)

func TestRunnable(t *testing.T) {
	t.Run("method value is usable as an OpFn", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		c := egtest.NewBuffer()
		var r eg.Runnable = c.Op('a')
		require.NoError(t, eg.Perform(ctx, r.Op, eg.Sequential(r.Op, c.Op('b'))))
		require.Equal(t, []byte{'a', 'a', 'b'}, c.Current())
	})

	t.Run("can be embedded in another interface", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		type runner interface {
			eg.Runnable
		}

		c := egtest.NewBuffer()
		var r runner = c.Op('a')
		require.NoError(t, eg.Perform(ctx, r.Op))
		require.Equal(t, []byte{'a'}, c.Current())
	})
}
