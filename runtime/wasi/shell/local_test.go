package shell_test

import (
	"bytes"
	"fmt"
	"os/exec"
	"os/user"
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestLocalCommands(t *testing.T) {
	u := userx.CurrentUserOrDefault(user.User{Username: "egd"})

	type example struct {
		description string
		cmd         func(cmd shell.Command) shell.Command
		expectedout string
		expectederr string
		err         error
	}

	examples := []example{
		{
			description: "example 1 - hello world",
			cmd:         func(cmd shell.Command) shell.Command { return cmd.New("echo \"hello world\"") },
			expectedout: "hello world\n",
		},
		{
			description: "example 2 - multiple attempts",
			cmd:         func(cmd shell.Command) shell.Command { return cmd.New("echo \"hello world\"; false").Attempts(2) },
			expectedout: "hello world\nhello world\n",
			err:         &exec.ExitError{},
		},
		{
			description: "example 3 - environment variables",
			cmd:         func(cmd shell.Command) shell.Command { return cmd.New("echo \"${FOO}\"").Environ("FOO", "hello world") },
			expectedout: "hello world\n",
		},
	}

	ctx, done := testx.Context(t)
	defer done()

	for _, e := range examples {
		t.Run(e.description, func(t *testing.T) {
			var bufout, buferr bytes.Buffer
			runtime := shell.NewLocalStd(&bufout, &buferr).As(u.Username)
			if e.err != nil {
				err := shell.Run(ctx, e.cmd(runtime))
				require.ErrorAs(t, err, &e.err, fmt.Sprintf("%T - %v\n", errorsx.Cause(err), err))
			} else {
				require.NoError(t, shell.Run(ctx, e.cmd(runtime)))
			}
			require.Equal(t, e.expectedout, bufout.String())
			require.Equal(t, e.expectederr, buferr.String())
		})
	}
}
