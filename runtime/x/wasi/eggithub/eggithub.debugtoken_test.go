package eggithub

import (
	"context"
	"errors"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestDebugToken(t *testing.T) {
	t.Setenv(_eg.EnvGithubToken, "token")

	t.Run("reports the authentication status of the token", func(t *testing.T) {
		cmd := shell.NewLocal()
		rec := shell.NewRecorder(&cmd)

		require.NoError(t, debugtoken(cmd)(t.Context(), nil))
		require.Len(t, rec.Results(), 1)
		require.Contains(t, rec.Result(), "gh auth status")
		require.NotContains(t, rec.Result(), "--show-token")
	})

	t.Run("a failing status is not an error", func(t *testing.T) {
		cmd := shell.NewLocal().UnsafeExec(func(context.Context, string, []string, string, []string) error {
			return errors.New("HTTP 403: Resource not accessible by integration (https://api.github.com/user)")
		})

		require.NoError(t, debugtoken(cmd)(t.Context(), nil))
	})
}
