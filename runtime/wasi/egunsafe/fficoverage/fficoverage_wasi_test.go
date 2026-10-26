package fficoverage

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/stretchr/testify/require"
)

// sessions opened from the guest must be released on close: the server only has a few
// pooled connections, so more sequential sessions than that all have to complete.
func TestWasiSequentialSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a wasip1 guest")
	}

	const sessions = "12"

	wasm := interptest.Build(t, "./runtime/wasi/egunsafe/fficoverage/.fixtures/sessions")

	for _, mode := range []string{"dial", "ping", "report"} {
		t.Run(mode, func(t *testing.T) {
			a := interptest.StartAnalytics(t, duckproxyserver.WithAcquireTimeout(5*time.Second))

			require.NoError(t, interptest.Run(t, wasm, a.RuntimeDir, time.Minute, "SESSIONS_MODE="+mode, "SESSIONS_COUNT="+sessions))

			if mode != "report" {
				return
			}

			var n int
			require.NoError(t, a.DB.QueryRow(`SELECT count(*) FROM 'eg.metrics.coverage'`).Scan(&n))
			require.Equal(t, 12*128, n)
		})
	}
}
