package ffiexec

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestStream(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a wasip1 guest")
	}

	wasm := interptest.Build(t, "./runtime/wasi/egunsafe/ffiexec/.fixtures/stream")

	serve := func(t *testing.T) string {
		// unix socket paths are length limited, t.TempDir can exceed it.
		rundir, err := os.MkdirTemp("", "ffiexec")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(rundir) })

		l, err := net.Listen("unix", filepath.Join(rundir, eg.SocketControl))
		require.NoError(t, err)
		srv := grpc.NewServer()
		execproxy.NewProcProxy(t.TempDir(), rundir, os.Environ()).Bind(srv)
		go srv.Serve(l)
		t.Cleanup(srv.Stop)
		return rundir
	}

	for _, mode := range []string{"output", "exit", "stdin", "pipeline"} {
		t.Run(mode, func(t *testing.T) {
			require.NoError(t, interptest.Run(t, wasm, serve(t), time.Minute, "STREAM_MODE="+mode))
		})
	}
}
