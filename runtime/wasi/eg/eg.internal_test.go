package eg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainerRunnerClone(t *testing.T) {
	t.Run("clone changes should not impact original", func(t *testing.T) {
		o := Container("derp")
		dup := o.Clone().OptionEnv("FOO", "BAR").Command("echo ${FOO}")
		require.Empty(t, o.options)
		require.Empty(t, o.cmd)
		require.Len(t, dup.options, 1)
		require.Equal(t, []string{"echo", "${FOO}"}, dup.cmd)
	})
}
