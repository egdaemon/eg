package eg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPattern(t *testing.T) {
	t.Setenv(EnvComputeRunID, "run-1")
	t.Setenv(EnvComputeAccountID, "acct-1")
	t.Setenv(EnvComputeWorkloadName, "build")
	t.Setenv(EnvComputeArch, "amd64")
	t.Setenv(EnvComputeOS, "linux")

	require.Equal(t, "run-1", Pattern("%eg.run.id%"))
	require.Equal(t, "acct-1", Pattern("%eg.account.id%"))
	require.Equal(t, "build", Pattern("%eg.workload%"))
	require.Equal(t, "amd64", Pattern("%eg.host.arch%"))
	require.Equal(t, "linux", Pattern("%eg.host.os%"))
	require.Equal(t, "build-linux-amd64-run-1", Pattern("%eg.workload%-%eg.host.os%-%eg.host.arch%-%eg.run.id%"))
	require.Equal(t, "run-1.run-1", Pattern("%eg.run.id%.%eg.run.id%"))
	// unknown tokens and patterns without tokens are untouched.
	require.Equal(t, "%eg.unknown%", Pattern("%eg.unknown%"))
	require.Equal(t, "plain", Pattern("plain"))
	require.Equal(t, "", Pattern(""))
}

func TestPatternUnset(t *testing.T) {
	t.Setenv(EnvComputeRunID, "")
	require.Equal(t, "v", Pattern("v%eg.run.id%"))
}
