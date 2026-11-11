package actl_test

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/docker/go-units"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdtestx/v2"
	"github.com/egdaemon/eg/cmd/eg/actl"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/netx"
	"github.com/egdaemon/eg/runners"
	"github.com/stretchr/testify/require"
)

func TestBootstrapEnvRunner(t *testing.T) {
	t.Run("defaults resolve to the host's own resources", func(t *testing.T) {
		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvRunner{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil))

		require.NoError(t, harness.Run())
		require.Contains(t, out.String(), fmt.Sprintf("EG_RUNNER_CPU=%d", runtime.NumCPU()))
		require.Contains(t, out.String(), "EG_RUNNER_MEMORY=")
	})

	t.Run("memory is written quoted in a format podman accepts for --memory", func(t *testing.T) {
		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvRunner{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil))

		require.NoError(t, harness.Run("--memory=1536MiB"))
		require.Contains(t, out.String(), "EG_RUNNER_MEMORY=\"1536 MiB\"\n")

		// podman parses --memory with go-units.
		parsed, err := units.RAMInBytes("1536 MiB")
		require.NoError(t, err)
		require.EqualValues(t, 1536*bytesx.MiB, parsed)
	})

	t.Run("cores flag below the host's actual core count wins", func(t *testing.T) {
		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvRunner{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil))

		require.NoError(t, harness.Run("--cores=1"))
		require.Contains(t, out.String(), "EG_RUNNER_CPU=1")
	})

	t.Run("cores flag above the host's actual core count is capped at the real count", func(t *testing.T) {
		requested := uint64(runtime.NumCPU()) + 1000

		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvRunner{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil))

		require.NoError(t, harness.Run("--cores="+strconv.FormatUint(requested, 10)))
		require.Contains(t, out.String(), fmt.Sprintf("EG_RUNNER_CPU=%d", runtime.NumCPU()))
	})

	t.Run("memory flag larger than the host's available memory is capped at what the host actually has", func(t *testing.T) {
		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvRunner{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil))

		require.NoError(t, harness.Run("--memory=2Pi"))
		// requesting far more than the host has must not simply echo the request back
		require.NotContains(t, out.String(), fmt.Sprintf("EG_RUNNER_MEMORY=%v", bytesx.Unit(2*bytesx.PiB)))
		require.Contains(t, out.String(), "EG_RUNNER_MEMORY=")
	})

	t.Run("vram flag larger than any detectable GPU's vram is capped at what was actually detected", func(t *testing.T) {
		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvRunner{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil))

		require.NoError(t, harness.Run("--vram=2Pi"))
		require.NotContains(t, out.String(), fmt.Sprintf("EG_RUNNER_VRAM=%v", bytesx.Unit(2*bytesx.PiB)))
		require.NotContains(t, out.String(), "2 PiB")
	})

	t.Run("vram is only written, and quoted, when a gpu is available", func(t *testing.T) {
		_, gpuvram, err := runners.DetectGPU()
		require.NoError(t, err)

		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvRunner{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil))

		require.NoError(t, harness.Run())
		if gpuvram == 0 {
			require.NotContains(t, out.String(), "EG_RUNNER_VRAM")
			return
		}

		require.Contains(t, out.String(), fmt.Sprintf("EG_RUNNER_VRAM=\"%s\"\n", errorsx.Must(gpuvram.MarshalText())))
	})

	t.Run("the default network interface is written with or without lan", func(t *testing.T) {
		if _, err := netx.DefaultRouteInterface(); err != nil {
			t.Skip("host has no default route:", err)
		}

		iface, err := netx.DefaultRouteInterface()
		require.NoError(t, err)

		for _, args := range [][]string{nil, {"--lan"}} {
			var out bytes.Buffer
			harness := cmdtestx.New(t, &actl.BootstrapEnvRunner{}).Options(
				cmdopts.RuntimeResources{}.KongVars(),
				kong.Writers(&out, nil))

			require.NoError(t, harness.Run(args...))
			require.Contains(t, out.String(), fmt.Sprintf("EG_RUNNER_DEFAULT_NETWORK_INTERFACE=\"%s\"\n", iface))
		}
	})

	t.Run("the runner network is only set when asked for", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			args     []string
			contains []string
			absent   []string
		}{
			{name: "defaults keep the unit's bridge", absent: []string{"EG_RUNNER_NETWORK"}},
			{name: "lan switches to egnet-lan", args: []string{"--lan"}, contains: []string{"EG_RUNNER_NETWORK=egnet-lan"}},
			{name: "negated keeps the bridge", args: []string{"--no-lan"}, absent: []string{"EG_RUNNER_NETWORK"}},
		} {
			t.Run(c.name, func(t *testing.T) {
				var out bytes.Buffer
				harness := cmdtestx.New(t, &actl.BootstrapEnvRunner{}).Options(
					cmdopts.RuntimeResources{}.KongVars(),
					kong.Writers(&out, nil))

				require.NoError(t, harness.Run(c.args...))
				for _, v := range c.contains {
					require.Contains(t, out.String(), v)
				}
				for _, v := range c.absent {
					require.NotContains(t, out.String(), v)
				}
			})
		}
	})
}
