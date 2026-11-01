package eg

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRuntimeRef(t *testing.T) {
	t.Run("id is derived from the function pointer", func(t *testing.T) {
		fn := func(context.Context, Op) error { return nil }
		require.Equal(t, ref(fn).ID(), ref(fn).ID())
		require.NotEmpty(t, ref(fn).ID())
	})

	t.Run("op info describes the function", func(t *testing.T) {
		fn := func(context.Context, Op) error { return nil }
		info := ref(fn).(runtimeref).OpInfo(time.Now(), nil, []string{"a"})
		require.NotNil(t, info)
		require.NotEmpty(t, info.Name)
		require.Contains(t, info.Module, "eg.runtimeref.internal_test.go")
		require.Equal(t, []string{"a"}, info.Path)
	})
}
