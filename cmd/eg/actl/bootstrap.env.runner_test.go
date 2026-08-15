package actl_test

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/egdaemon/eg/cmd/eg/actl"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/stretchr/testify/require"
)

func TestBootstrapEnvRunner(t *testing.T) {
	t.Run("defaults resolve to the host's own resources", func(t *testing.T) {
		var out bytes.Buffer
		genparser := cmdtestx.Genparser(actl.BootstrapEnvRunner{},
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil),
		)

		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command"))
		require.Contains(t, out.String(), fmt.Sprintf("EG_RUNNER_CPU=%d", runtime.NumCPU()))
		require.Contains(t, out.String(), "EG_RUNNER_MEMORY=")
		require.Contains(t, out.String(), "EG_RUNNER_VRAM=")
	})

	t.Run("cores flag below the host's actual core count wins", func(t *testing.T) {
		var out bytes.Buffer
		genparser := cmdtestx.Genparser(actl.BootstrapEnvRunner{},
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil),
		)

		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", "--cores=1"))
		require.Contains(t, out.String(), "EG_RUNNER_CPU=1")
	})

	t.Run("cores flag above the host's actual core count is capped at the real count", func(t *testing.T) {
		requested := uint64(runtime.NumCPU()) + 1000

		var out bytes.Buffer
		genparser := cmdtestx.Genparser(actl.BootstrapEnvRunner{},
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil),
		)

		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", "--cores="+strconv.FormatUint(requested, 10)))
		require.Contains(t, out.String(), fmt.Sprintf("EG_RUNNER_CPU=%d", runtime.NumCPU()))
	})

	t.Run("memory flag larger than the host's available memory is capped at what the host actually has", func(t *testing.T) {
		var out bytes.Buffer
		genparser := cmdtestx.Genparser(actl.BootstrapEnvRunner{},
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil),
		)

		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", "--memory=2Pi"))
		// requesting far more than the host has must not simply echo the request back
		require.NotContains(t, out.String(), fmt.Sprintf("EG_RUNNER_MEMORY=%v", bytesx.Unit(2*bytesx.PiB)))
		require.Contains(t, out.String(), "EG_RUNNER_MEMORY=")
	})

	t.Run("vram flag larger than any detectable GPU's vram is capped at what was actually detected", func(t *testing.T) {
		var out bytes.Buffer
		genparser := cmdtestx.Genparser(actl.BootstrapEnvRunner{},
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Writers(&out, nil),
		)

		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", "--vram=2Pi"))
		require.NotContains(t, out.String(), fmt.Sprintf("EG_RUNNER_VRAM=%v", bytesx.Unit(2*bytesx.PiB)))
		require.Contains(t, out.String(), "EG_RUNNER_VRAM=")
	})
}
