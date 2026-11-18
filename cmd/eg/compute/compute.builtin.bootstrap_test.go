package compute

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModuleArgs(t *testing.T) {
	t.Run("the arguments are those after bootstrap", func(t *testing.T) {
		require.Equal(t,
			[]string{"-vv", "release/egcoreutils"},
			moduleargs([]string{"eg", "-v", "compute", "builtin", "bootstrap", "-vv", "release/egcoreutils"}),
		)
	})

	t.Run("flags applied by the bootstrap are dropped with their values", func(t *testing.T) {
		require.Equal(t,
			[]string{"-vv", "--auto-patch", "-e", "A=1", "release/egcoreutils"},
			moduleargs([]string{
				"eg", "compute", "builtin", "bootstrap",
				"--gcp-auto", "--secret", "gcpsm://p/s", "-vv", "--secret=gcpsm://p/t", "--privileged",
				"--gcp", "/adc.json", "--gcp=/adc.json", "--dirty", "--wayland", "--ports", "80", "--cores=4", "--memory", "8GiB",
				"--auto-patch", "-e", "A=1", "release/egcoreutils",
			}),
		)
	})
}
