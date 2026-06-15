package runners

// GPUDeviceSpec maps a kernel driver name to its CDI device specification.
// returns an empty string for unknown/unsupported drivers.
func GPUDeviceSpec(driver string) string {
	switch driver {
	case "amdgpu":
		return "amd.com/gpu=all"
	default:
		return ""
	}
}
