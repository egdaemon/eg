package shell_test

import (
	"fmt"
	"os/user"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestShellCommands(t *testing.T) {
	current, err := user.Current()
	require.NoError(t, err)
	group := envx.String(current.Username, eg.EnvComputeDefaultGroup)

	type example struct {
		cmd      shell.Command
		expected string
	}
	examples := []example{
		{
			cmd:      shell.New("echo \"hello world\"").User("derp").Group("derp"),
			expected: "::sudo:-E -H -u derp -g derp bash -o pipefail -c echo \"hello world\"",
		},
		{
			cmd:      shell.New("psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\""),
			expected: fmt.Sprintf("::sudo:-E -H -u %s -g %s bash -o pipefail -c psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\"", current.Username, group),
		},
	}

	ctx, done := testx.Context(t)
	defer done()

	for _, e := range examples {
		rec := shell.NewRecorder(&e.cmd)
		require.NoError(t, shell.Run(ctx, e.cmd))
		require.Equal(t, rec.Result(), e.expected)
	}
}
