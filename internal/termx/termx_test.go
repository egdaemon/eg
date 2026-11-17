package termx_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
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

func TestIsTTY(t *testing.T) {
	null, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer null.Close()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()
	defer w.Close()

	require.False(t, termx.IsTTY(null))
	require.False(t, termx.IsTTY(r))
	require.False(t, termx.IsTTY(w))
	require.False(t, termx.IsTTY(nil))
	require.False(t, termx.IsTTY())
	require.False(t, termx.IsTTY(null, r))
	require.False(t, termx.IsTTY(io.Discard))
}
