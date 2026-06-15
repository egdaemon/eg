package runners

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGPUDeviceSpec(t *testing.T) {
	tests := []struct {
		driver   string
		expected string
	}{
		{driver: "amdgpu", expected: "amd.com/gpu=all"},
		{driver: "nvidia", expected: "nvidia.com/gpu=all"},
		{driver: "nouveau", expected: ""},
		{driver: "i915", expected: ""},
		{driver: "", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.driver, func(t *testing.T) {
			require.Equal(t, tc.expected, GPUDeviceSpec(tc.driver))
		})
	}
}
