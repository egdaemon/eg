package daemons_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/egdaemon/eg/computeapi"

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

		enqresp := computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 1},
			AccessToken: "tok",
		}
		mimetype, body, err := computeapi.NewWorkloadRequestToken(&enqresp, "candidate", strings.NewReader(""))
		require.NoError(t, err)
		defer body.Close()

		r := httptest.NewRequest(http.MethodPost, "/c/enqueue", body)
		r.Header.Set("Content-Type", mimetype)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusAccepted, w.Code)

		var resp computeapi.Enqueued
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, enqresp.Enqueued.Id, resp.Id)

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, "candidate", runners.ReadCandidateToken(filepath.Join(h.Dirs.Queued, entries[0].Name())))
	})

	t.Run("requests without a candidate token are rejected without touching the spool", func(t *testing.T) {
		h := &daemons.EnqueueHandler{
			Dirs: runners.NewSpoolDir(t.TempDir()),
			RM:   runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10}),
		}

		enqresp := computeapi.EnqueuedDequeueResponse{
			Enqueued: &computeapi.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 1},
		}
		mimetype, body, err := computeapi.NewWorkloadRequest(&enqresp, strings.NewReader(""))
		require.NoError(t, err)
		defer body.Close()

		r := httptest.NewRequest(http.MethodPost, "/c/enqueue", body)
		r.Header.Set("Content-Type", mimetype)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)

		entries, err := os.ReadDir(h.Dirs.Downloading)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("requests that would exceed target load are rejected without touching the spool", func(t *testing.T) {
		h := &daemons.EnqueueHandler{
			Dirs: runners.NewSpoolDir(t.TempDir()),
			RM:   runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10}),
		}

		enqresp := computeapi.EnqueuedDequeueResponse{
			Enqueued: &computeapi.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 9},
		}
		mimetype, body, err := computeapi.NewWorkloadRequest(&enqresp, strings.NewReader(""))
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

	t.Run("duplicate pushes of a queued or running id are accepted without touching the spool", func(t *testing.T) {
		h := &daemons.EnqueueHandler{
			Dirs: runners.NewSpoolDir(t.TempDir()),
			RM:   runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10}),
		}

		enqresp := computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 1},
			AccessToken: "tok",
		}

		for i := 0; i < 2; i++ {
			mimetype, body, err := computeapi.NewWorkloadRequestToken(&enqresp, "candidate", strings.NewReader(""))
			require.NoError(t, err)
			defer body.Close()

			r := httptest.NewRequest(http.MethodPost, "/c/enqueue", body)
			r.Header.Set("Content-Type", mimetype)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, http.StatusAccepted, w.Code)
		}

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		downloading, err := os.ReadDir(h.Dirs.Downloading)
		require.NoError(t, err)
		require.Empty(t, downloading)

		require.NoError(t, os.Rename(filepath.Join(h.Dirs.Queued, entries[0].Name()), filepath.Join(h.Dirs.Running, entries[0].Name())))

		mimetype, body, err := computeapi.NewWorkloadRequestToken(&enqresp, "candidate", strings.NewReader(""))
		require.NoError(t, err)
		defer body.Close()

		r := httptest.NewRequest(http.MethodPost, "/c/enqueue", body)
		r.Header.Set("Content-Type", mimetype)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusAccepted, w.Code)

		entries, err = os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("concurrent pushes of the same id queue it once", func(t *testing.T) {
		h := &daemons.EnqueueHandler{
			Dirs: runners.NewSpoolDir(t.TempDir()),
			RM:   runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10}),
		}

		enqresp := computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 1},
			AccessToken: "tok",
		}

		var wg sync.WaitGroup
		codes := make([]int, 8)
		for i := range codes {
			mimetype, body, err := computeapi.NewWorkloadRequestToken(&enqresp, "candidate", strings.NewReader(""))
			require.NoError(t, err)
			defer body.Close()

			r := httptest.NewRequest(http.MethodPost, "/c/enqueue", body)
			r.Header.Set("Content-Type", mimetype)
			w := httptest.NewRecorder()
			wg.Add(1)
			go func() {
				defer wg.Done()
				h.ServeHTTP(w, r)
				codes[i] = w.Code
			}()
		}
		wg.Wait()

		for _, code := range codes {
			require.Equal(t, http.StatusAccepted, code)
		}

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		downloading, err := os.ReadDir(h.Dirs.Downloading)
		require.NoError(t, err)
		require.Empty(t, downloading)
	})

	t.Run("a stale download with no lock holder is discarded and the push proceeds", func(t *testing.T) {
		h := &daemons.EnqueueHandler{
			Dirs: runners.NewSpoolDir(t.TempDir()),
			RM:   runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10}),
		}

		enqresp := computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), VcsUri: "https://example.com/repo.git", VcsCommit: "deadbeef", Cores: 1},
			AccessToken: "tok",
		}

		stale := filepath.Join(h.Dirs.Downloading, runners.Queued().Dirname(uuid.Must(uuid.FromString(enqresp.Enqueued.Id))))
		require.NoError(t, os.MkdirAll(stale, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(stale, "partial"), []byte("x"), 0600))

		mimetype, body, err := computeapi.NewWorkloadRequestToken(&enqresp, "candidate", strings.NewReader(""))
		require.NoError(t, err)
		defer body.Close()

		r := httptest.NewRequest(http.MethodPost, "/c/enqueue", body)
		r.Header.Set("Content-Type", mimetype)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusAccepted, w.Code)

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		_, err = os.Stat(filepath.Join(h.Dirs.Queued, entries[0].Name(), "partial"))
		require.True(t, os.IsNotExist(err))
	})
}
