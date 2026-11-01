package eg_test

import (
	"context"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/stretchr/testify/require"
)

func TestFn(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	var called bool
	var r eg.Runnable = eg.Fn(func(context.Context, eg.Op) error {
		called = true
		return nil
	})
	require.NoError(t, eg.Perform(ctx, r.Op))
	require.True(t, called)
}
