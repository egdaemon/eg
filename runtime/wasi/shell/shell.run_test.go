package shell_test

import (
	"strings"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Run("defaults to bash in the module", func(t *testing.T) {
		t.Setenv(_eg.EnvExperimentalGuestShell, "")
		cmd := shell.New("echo hello")
		rec := shell.NewRecorder(&cmd)
		require.NoError(t, shell.Run(t.Context(), cmd))
		require.True(t, strings.HasSuffix(rec.Result(), "bash -o pipefail -c echo hello"), rec.Result())
	})
}
