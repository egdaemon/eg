//go:build linux

package runners

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestAgentOptionHostOSUnmask(t *testing.T) {
	t.Run("podman's default masking is left alone when the variable is unset", func(t *testing.T) {
		t.Setenv(eg.EnvComputeRunnerUnmask, "")

		a := langx.Clone(Agent{}, AgentOptionHostOS())

		require.NotContains(t, a.literals, "unmask=/proc/*")
		for _, literal := range a.literals {
			require.NotContains(t, literal, "unmask=")
		}
	})

	t.Run("proc is unmasked when the variable is true", func(t *testing.T) {
		t.Setenv(eg.EnvComputeRunnerUnmask, "true")

		a := langx.Clone(Agent{}, AgentOptionHostOS())

		require.Contains(t, a.literals, "unmask=/proc/*")
	})

	t.Run("the unmask follows its --security-opt", func(t *testing.T) {
		t.Setenv(eg.EnvComputeRunnerUnmask, "true")

		a := langx.Clone(Agent{}, AgentOptionHostOS())

		for i, literal := range a.literals {
			if literal == "unmask=/proc/*" {
				require.Greater(t, i, 0)
				require.Equal(t, "--security-opt", a.literals[i-1])
				return
			}
		}

		require.Fail(t, "unmask option not found", "%v", a.literals)
	})

	t.Run("the existing nesting options are still present", func(t *testing.T) {
		t.Setenv(eg.EnvComputeRunnerUnmask, "")

		a := langx.Clone(Agent{}, AgentOptionHostOS())

		require.Contains(t, a.literals, "SYS_ADMIN")
		require.Contains(t, a.literals, "/dev/fuse")
	})
}

func TestAgentOptionHostOSOverrides(t *testing.T) {
	t.Run("the defaults are unchanged", func(t *testing.T) {
		t.Setenv(eg.EnvComputePodmanUsernamespace, "")
		t.Setenv(eg.EnvComputePodmanCapNetwork, "")
		t.Setenv(eg.EnvComputePodmanCapAdmin, "")
		t.Setenv(eg.EnvComputePodmanDeviceFuse, "")
		t.Setenv(eg.EnvComputePodmanDeviceTun, "")
		t.Setenv(eg.EnvComputePodmanSecurityLabel, "")
		t.Setenv(eg.EnvComputePodmanPidsLimit, "")
		t.Setenv(eg.EnvComputeRunnerUnmask, "")

		a := langx.Clone(Agent{}, AgentOptionHostOS())

		require.Equal(t, []string{
			"--userns", "host",
			"--cap-add", "NET_ADMIN",
			"--cap-add", "SYS_ADMIN",
			"--device", "/dev/fuse",
			"--device", "/dev/net/tun",
			"--security-opt", "label=disable",
			"--pids-limit", "-1",
		}, a.literals)
	})

	t.Run("the value variables override the value of their own flag", func(t *testing.T) {
		t.Setenv(eg.EnvComputePodmanUsernamespace, "keep-id")
		t.Setenv(eg.EnvComputePodmanSecurityLabel, "label=nested")
		t.Setenv(eg.EnvComputePodmanPidsLimit, "4096")
		t.Setenv(eg.EnvComputeRunnerUnmask, "")

		a := langx.Clone(Agent{}, AgentOptionHostOS())

		require.Equal(t, []string{
			"--userns", "keep-id",
			"--cap-add", "NET_ADMIN",
			"--cap-add", "SYS_ADMIN",
			"--device", "/dev/fuse",
			"--device", "/dev/net/tun",
			"--security-opt", "label=nested",
			"--pids-limit", "4096",
		}, a.literals)
	})

	t.Run("the capability and device variables toggle their flag", func(t *testing.T) {
		t.Setenv(eg.EnvComputePodmanCapNetwork, "false")
		t.Setenv(eg.EnvComputePodmanCapAdmin, "true")
		t.Setenv(eg.EnvComputePodmanDeviceFuse, "false")
		t.Setenv(eg.EnvComputePodmanDeviceTun, "")
		t.Setenv(eg.EnvComputeRunnerUnmask, "")

		a := langx.Clone(Agent{}, AgentOptionHostOS())

		require.Equal(t, []string{
			"--userns", "host",
			"--cap-add", "SYS_ADMIN",
			"--device", "/dev/net/tun",
			"--security-opt", "label=disable",
			"--pids-limit", "-1",
		}, a.literals)
	})
}
