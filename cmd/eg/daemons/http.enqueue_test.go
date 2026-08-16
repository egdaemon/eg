package daemons_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestEnqueueHandler(t *testing.T) {
	t.Run("accepted requests are spooled for compile", func(t *testing.T) {
		h := &daemons.EnqueueHandler{
			Dirs: runners.NewSpoolDir(t.TempDir()),
			RM:   runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10}),
		}

		enqresp := runners.EnqueuedDequeueResponse{
			Enqueued:    &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 1},
			AccessToken: "tok",
		}
		mimetype, body, err := runners.NewWorkloadRequest(&enqresp, strings.NewReader(""))
		require.NoError(t, err)
		defer body.Close()

		r := httptest.NewRequest(http.MethodPost, "/c/enqueue", body)
		r.Header.Set("Content-Type", mimetype)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusAccepted, w.Code)

		var resp runners.Enqueued
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, enqresp.Enqueued.Id, resp.Id)

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Len(t, entries, 1)
	})

	t.Run("requests that would exceed target load are rejected without touching the spool", func(t *testing.T) {
		h := &daemons.EnqueueHandler{
			Dirs: runners.NewSpoolDir(t.TempDir()),
			RM:   runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10}),
		}

		enqresp := runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 9},
		}
		mimetype, body, err := runners.NewWorkloadRequest(&enqresp, strings.NewReader(""))
		require.NoError(t, err)
		defer body.Close()

		r := httptest.NewRequest(http.MethodPost, "/c/enqueue", body)
		r.Header.Set("Content-Type", mimetype)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusConflict, w.Code)

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Empty(t, entries)

		entries, err = os.ReadDir(h.Dirs.Downloading)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("malformed bodies are rejected", func(t *testing.T) {
		h := &daemons.EnqueueHandler{
			Dirs: runners.NewSpoolDir(t.TempDir()),
			RM:   runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10}),
		}

		r := httptest.NewRequest(http.MethodPost, "/c/enqueue", strings.NewReader("not json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
