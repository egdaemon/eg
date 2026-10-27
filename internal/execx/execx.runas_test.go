package execx

import (
	"os"
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunAs(t *testing.T) {
	t.Run("unknown user runs directly", func(t *testing.T) {
		cmd := RunAs(t.Context(), "eg-no-such-user", "echo", "hi")
		require.Equal(t, []string{"echo", "hi"}, cmd.Args)
	})

	t.Run("current user runs directly", func(t *testing.T) {
		u, err := user.Current()
		require.NoError(t, err)

		cmd := RunAs(t.Context(), u.Username, "echo", "hi")
		require.Equal(t, []string{"echo", "hi"}, cmd.Args)
	})

	t.Run("different user is wrapped in sudo", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root is the current user")
		}
		u, err := user.Lookup("root")
		if err != nil {
			t.Skip("root user unavailable")
		}

		cmd := RunAs(t.Context(), "root", "echo", "hi")
		require.Equal(t, []string{"sudo", "-u", u.Username, "-g", u.Username, "--", "echo", "hi"}, cmd.Args)
	})
}
