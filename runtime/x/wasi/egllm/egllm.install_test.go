package egllm_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestInstall(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	// the unit is cloned into the ephemeral directory as a read-only file, keep each run's separate.
	t.Setenv("TMPDIR", t.TempDir())

	var (
		cmd = shell.NewLocal()
		rec = shell.NewRecorder(&cmd)
	)

	require.NoError(t, egllm.Install(cmd)(ctx, nil))
	require.Equal(t, []string{
		"::bash:-c install -m 0644 " + egenv.EphemeralDirectory("llama-server.service") + " /etc/systemd/system/llama-server.service",
		"::bash:-c systemctl daemon-reload",
	}, rec.Results())
}
