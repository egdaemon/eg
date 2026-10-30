package egenv_test

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestPattern(t *testing.T) {
	t.Setenv(_eg.EnvComputeRunID, "run-1")
	t.Setenv(_eg.EnvComputeAccountID, "acct-1")
	t.Setenv(_eg.EnvComputeWorkloadName, "build")
	t.Setenv(_eg.EnvComputeArch, "amd64")
	t.Setenv(_eg.EnvComputeOS, "linux")

	require.Equal(t, "run-1", egenv.Pattern("%eg.run.id%"))
	require.Equal(t, "acct-1", egenv.Pattern("%eg.account.id%"))
	require.Equal(t, "build", egenv.Pattern("%eg.workload%"))
	require.Equal(t, "amd64", egenv.Pattern("%eg.host.arch%"))
	require.Equal(t, "linux", egenv.Pattern("%eg.host.os%"))
	require.Equal(t, "build-linux-amd64-run-1", egenv.Pattern("%eg.workload%-%eg.host.os%-%eg.host.arch%-%eg.run.id%"))
	require.Equal(t, "run-1.run-1", egenv.Pattern("%eg.run.id%.%eg.run.id%"))
	// unknown tokens and patterns without tokens are untouched.
	require.Equal(t, "%eg.unknown%", egenv.Pattern("%eg.unknown%"))
	require.Equal(t, "plain", egenv.Pattern("plain"))
	require.Equal(t, "", egenv.Pattern(""))
}

func TestPatternDefaults(t *testing.T) {
	t.Setenv(_eg.EnvComputeRunID, "")
	t.Setenv(_eg.EnvComputeWorkloadName, "")
	require.Equal(t, "00000000-0000-0000-0000-000000000000", egenv.Pattern("%eg.run.id%"))
	require.Equal(t, "v", egenv.Pattern("v%eg.workload%"))
}
