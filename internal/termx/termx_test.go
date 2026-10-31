package termx_test

import (
	"errors"
	"io/fs"
	"os/exec"
	"testing"

	"github.com/egdaemon/eg/internal/termx"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	// exec.Cmd stands in for a prompt, it has the same Run() error shape.
	err := termx.Run(exec.Command("/nonexistent/prompt"))
	require.ErrorIs(t, err, termx.ErrNoTTY)
	require.ErrorIs(t, err, fs.ErrNotExist)

	err = termx.Run(exec.Command("false"))
	require.Error(t, err)
	require.False(t, errors.Is(err, termx.ErrNoTTY))

	require.NoError(t, termx.Run(exec.Command("true")))
}
