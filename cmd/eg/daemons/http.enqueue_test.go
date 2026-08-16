package daemons_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/runners"
	"github.com/stretchr/testify/require"
)

func newEnqueueHandler(t *testing.T, limits runners.RuntimeResources) *daemons.EnqueueHandler {
	t.Helper()
	return &daemons.EnqueueHandler{
		Dirs: runners.NewSpoolDir(t.TempDir()),
		RM:   runners.NewResourceManager(limits),
	}
}

func doEnqueue(h *daemons.EnqueueHandler, req runners.CompileRequest) *httptest.ResponseRecorder {
	encoded, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/c/enqueue", bytes.NewReader(encoded))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestEnqueueHandler(t *testing.T) {
	t.Run("accepted requests are spooled for compile", func(t *testing.T) {
		h := newEnqueueHandler(t, runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10})

		w := doEnqueue(h, runners.CompileRequest{VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 1})
		require.Equal(t, http.StatusAccepted, w.Code)

		var resp map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.NotEmpty(t, resp["id"])

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Len(t, entries, 1)
	})

	t.Run("requests that would exceed target load are rejected without touching the spool", func(t *testing.T) {
		h := newEnqueueHandler(t, runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10})

		w := doEnqueue(h, runners.CompileRequest{VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 9})
		require.Equal(t, http.StatusConflict, w.Code)

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Empty(t, entries)

		entries, err = os.ReadDir(h.Dirs.Downloading)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("malformed bodies are rejected", func(t *testing.T) {
		h := newEnqueueHandler(t, runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10})

		r := httptest.NewRequest(http.MethodPost, "/c/enqueue", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
