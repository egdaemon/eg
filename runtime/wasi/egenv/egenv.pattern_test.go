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
	t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/ws")
	t.Setenv(_eg.EnvComputeWorkingDirectory, "/wd")
	t.Setenv(_eg.EnvComputeCacheDirectory, "/cache")
	t.Setenv("TMPDIR", "/tmp/example")

	require.Equal(t, "run-1", egenv.Pattern("%eg.run.id%"))
	require.Equal(t, "acct-1", egenv.Pattern("%eg.account.id%"))
	require.Equal(t, "build", egenv.Pattern("%eg.workload%"))
	require.Equal(t, "amd64", egenv.Pattern("%eg.host.arch%"))
	require.Equal(t, "linux", egenv.Pattern("%eg.host.os%"))
	require.Equal(t, "/ws", egenv.Pattern("%eg.workspace%"))
	require.Equal(t, "/wd", egenv.Pattern("%eg.working%"))
	require.Equal(t, "/cache", egenv.Pattern("%eg.cache%"))
	require.Equal(t, "/tmp/example", egenv.Pattern("%eg.tmp%"))
	require.Equal(t, "/ws/eg.rb", egenv.Pattern("%eg.workspace%/eg.rb"))
	require.Equal(t, "run-1", egenv.Pattern("{eg.run.id}"))
	require.Equal(t, "/ws/eg.rb", egenv.Pattern("{eg.workspace}/eg.rb"))
	require.Equal(t, "{eg.unknown}", egenv.Pattern("{eg.unknown}"))
	require.Equal(t, "build-linux-amd64-run-1", egenv.Pattern("%eg.workload%-%eg.host.os%-%eg.host.arch%-%eg.run.id%"))
	require.Equal(t, "run-1.run-1", egenv.Pattern("%eg.run.id%.%eg.run.id%"))
	// patterns without tokens are untouched.
	require.Equal(t, "plain", egenv.Pattern("plain"))
	require.Equal(t, "", egenv.Pattern(""))
}

func TestPatternDefaults(t *testing.T) {
	t.Setenv(_eg.EnvComputeRunID, "")
	t.Setenv(_eg.EnvComputeWorkloadName, "")
	require.Equal(t, "00000000-0000-0000-0000-000000000000", egenv.Pattern("%eg.run.id%"))
	require.Equal(t, "v", egenv.Pattern("v%eg.workload%"))
}

func TestPatternFormat(t *testing.T) {
	t.Setenv(_eg.EnvComputeRunID, "run-1")
	t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/ws")

	require.Equal(t, "/ws/foo", egenv.Pattern("%eg.workspace%/%s", "foo"))
	require.Equal(t, "run-1-3", egenv.Pattern("%eg.run.id%-%d", 3))
	// values containing percent signs are not interpreted as verbs.
	t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/100%/ws")
	require.Equal(t, "/100%/ws/foo", egenv.Pattern("%eg.workspace%/%s", "foo"))
}
