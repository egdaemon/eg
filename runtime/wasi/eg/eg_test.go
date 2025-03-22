package eg_test

import (
	"slices"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/stretchr/testify/require"
)

func TestParallelExecution(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	c := egtest.NewBuffer()
	require.NoError(t, eg.Perform(ctx, eg.Parallel(c.Op('a'), c.Op('b'), c.Op('c'), c.Op('d'))))
	res := c.Current()
	slices.Sort(res)
	require.Equal(t, []byte{'a', 'b', 'c', 'd'}, res)
}
