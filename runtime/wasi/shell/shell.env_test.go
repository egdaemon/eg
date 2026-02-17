package shell_test

import (
	"os"
	"strings"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	cmd := shell.Env().New("echo hello")
	rec := shell.NewRecorder(&cmd)
	require.NoError(t, shell.Run(ctx, cmd))

	result := rec.Result()
	for _, env := range os.Environ() {
		k := strings.SplitN(env, "=", 2)[0]
		require.Contains(t, result, k+"=", "expected environment variable %s to be present", k)
	}
}
