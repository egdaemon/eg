package runners

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestModuleArguments(t *testing.T) {
	base := []string{"module", "--directory", eg.DefaultWorkingDirectory(), "--moduledir", "/m", "/p"}

	t.Run("without arguments", func(t *testing.T) {
		require.Equal(t, base, modulearguments("", "/m", "/p", nil))
	})

	t.Run("arguments follow a double dash", func(t *testing.T) {
		require.Equal(t,
			append(append([]string(nil), base...), "--", "a b", "-x"),
			modulearguments("", "/m", "/p", []string{"a b", "-x"}),
		)
	})

	t.Run("machine option", func(t *testing.T) {
		var c machineconfig
		MachineOptionArgs("a", "b")(&c)
		MachineOptionArgs("c")(&c)
		require.Equal(t, []string{"a", "b", "c"}, c.args)
	})
}
