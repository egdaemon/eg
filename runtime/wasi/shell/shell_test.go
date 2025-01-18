package shell

import (
	"context"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

type commandresult struct {
	Directory string
	Binary    string
	Environ   []string
	Args      []string
}

func hijack(cmd Command) (Command, *commandresult) {
	r := &commandresult{}
	cmd.exec = r.exec
	return cmd, r
}

func (t *commandresult) exec(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
	t.Args = args
	t.Directory = dir
	t.Binary = cmd
	t.Environ = environ
	return nil
}

func TestShellCommands(t *testing.T) {
	type example struct {
		cmd       Command
		Directory string
		Binary    string
		Environ   []string
		Args      []string
	}
	examples := []example{
		{
			cmd:       New("echo \"hello world\"").User("derp").Group("derp"),
			Directory: "",
			Binary:    "sudo",
			Args:      []string{"-E", "-H", "-u", "derp", "-g", "derp", "bash", "-c", "echo \"hello world\""},
		},
		{
			cmd:       New("psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\""),
			Directory: "",
			Binary:    "sudo",
			Args:      []string{"-E", "-H", "-u", "egd", "-g", "egd", "bash", "-c", "psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\""},
		},
	}

	ctx, done := testx.Context(t)
	defer done()

	for _, e := range examples {
		cmd, result := hijack(e.cmd)
		require.NoError(t, Run(ctx, cmd))
		require.Equal(t, result.Directory, e.Directory)
		require.Equal(t, result.Binary, e.Binary)
		require.Equal(t, result.Environ, e.Environ)
		require.Equal(t, result.Args, e.Args)
	}
}
