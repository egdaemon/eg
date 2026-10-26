package runners

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestWorkloadCapacity(t *testing.T) {
	for _, c := range []struct {
		name     string
		value    string
		expected int
	}{
		{name: "defaults to a single workload when unset", expected: 1},
		{name: "uses the configured capacity", value: "4", expected: 4},
		{name: "zero is raised to the minimum of one", value: "0", expected: 1},
		{name: "negative is raised to the minimum of one", value: "-3", expected: 1},
		{name: "unparsable falls back to the default", value: "many", expected: 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(eg.EnvComputeWorkloadCapacity, c.value)
			require.Equal(t, c.expected, workloadcapacity())
		})
	}
}
