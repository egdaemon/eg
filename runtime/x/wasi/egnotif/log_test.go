package egnotif_test

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egnotif"
	"github.com/stretchr/testify/require"
)

func TestLog(t *testing.T) {
	t.Run("completed", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, egnotif.New(egnotif.Log(&buf, egnotif.LogOptions().Attrs("k", "v")...), egnotif.Options().Completed("ok")...).Fatalln(t.Context(), nil))
		require.Contains(t, buf.String(), "level=INFO")
		require.Contains(t, buf.String(), "msg=ok")
		require.Contains(t, buf.String(), "run_id=")
		require.Contains(t, buf.String(), "k=v")
	})

	t.Run("level option", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, egnotif.New(egnotif.Log(&buf, egnotif.LogOptions().Level(slog.LevelWarn)...)).Fatalln(t.Context(), nil))
		require.Contains(t, buf.String(), "level=WARN")
	})

	t.Run("failure is logged at error", func(t *testing.T) {
		var buf bytes.Buffer
		require.Error(t, egnotif.New(egnotif.Log(&buf, egnotif.LogOptions()...)).Fatalln(t.Context(), errors.New("boom")))
		require.Contains(t, buf.String(), "level=ERROR")
		require.Contains(t, buf.String(), "boom")
	})

	t.Run("invalid template is an error", func(t *testing.T) {
		var buf bytes.Buffer
		require.Error(t, egnotif.New(egnotif.Log(&buf), egnotif.Options().Completed("{{")...).Fatalln(t.Context(), nil))
	})
}
