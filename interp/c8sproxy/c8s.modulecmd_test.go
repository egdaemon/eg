package c8sproxy

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestModuleCmd(t *testing.T) {
	base := []string{"eg", "module", "--directory", eg.DefaultWorkingDirectory(), "--moduledir", "/m", eg.ModuleMount()}

	t.Run("without arguments", func(t *testing.T) {
		require.Equal(t, base, modulecmd("eg", "", "/m", nil))
	})

	t.Run("verbosity is a flag of eg", func(t *testing.T) {
		require.Equal(t, append([]string{"eg", "-vv"}, base[1:]...), modulecmd("eg", "-vv", "/m", nil))
	})

	t.Run("arguments follow a double dash and are not interpreted", func(t *testing.T) {
		require.Equal(t,
			append(append([]string(nil), base...), "--", "compute", "local", "-vv", "a b", ""),
			modulecmd("eg", "", "/m", []string{"compute", "local", "-vv", "a b", ""}),
		)
	})
}
