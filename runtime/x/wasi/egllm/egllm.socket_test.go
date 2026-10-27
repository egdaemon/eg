package egllm_test

import (
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestGenerateOverUnixSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "llama.sock")
	l, err := net.Listen("unix", sock)
	require.NoError(t, err)

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	})}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })

	t.Setenv(eg.EnvComputeLLMEndpoint, "unix://"+sock)
	require.Equal(t, "unix://"+sock, egllm.New().Host())

	generated, err := egllm.Generate(t.Context(), egllm.New(), "model", "say ok")
	require.NoError(t, err)
	require.Equal(t, "ok", generated)
}
