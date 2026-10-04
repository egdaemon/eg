package daemons_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

// fakereserver records the reservation request and returns a canned result.
type fakereserver struct {
	recorded *runners.EnqueuedDequeueResponse
	err      error
	called   bool
	id       string
	archive  string
	token    string
}

func (t *fakereserver) Reserve(ctx context.Context, enq *runners.Enqueued, archive io.Reader, token string) (*runners.EnqueuedDequeueResponse, error) {
	b, err := io.ReadAll(archive)
	if err != nil {
		return nil, err
	}

	t.called = true
	t.id, t.archive, t.token = enq.Id, string(b), token
	return t.recorded, t.err
}

func newRecorded() *runners.EnqueuedDequeueResponse {
	return &runners.EnqueuedDequeueResponse{
		Enqueued:    &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), Entry: "main.wasm"},
		AccessToken: "access-token",
	}
}

func newUploadRequest(t *testing.T, enq *runners.EnqueuedDequeueResponse, token string) *http.Request {
	t.Helper()

	mimetype, body, err := runners.NewDirectUpload(enq, token, strings.NewReader("kernel contents"), strings.NewReader("environ contents"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = body.Close() })

	r := httptest.NewRequest(http.MethodPost, "/c/upload", body)
	r.Header.Set("Content-Type", mimetype)
	return r
}

func requireEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestUploadHandler(t *testing.T) {
	t.Run("valid uploads are recorded and enqueued as recorded, no compile step", func(t *testing.T) {
		reserver := &fakereserver{recorded: newRecorded()}
		h := &daemons.UploadHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Reserver: reserver}
		h.RM = runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10})

		enqresp := runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), Entry: "main.wasm", Cores: 1},
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newUploadRequest(t, &enqresp, "candidate-token"))
		require.Equal(t, http.StatusAccepted, w.Code)

		require.True(t, reserver.called)
		require.Equal(t, enqresp.Enqueued.Id, reserver.id)
		require.Equal(t, "kernel contents", reserver.archive)
		require.Equal(t, "candidate-token", reserver.token)

		var resp runners.Enqueued
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, reserver.recorded.Enqueued.Id, resp.Id)

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		// spooled under the recorded id so completion cleans it up.
		dir := filepath.Join(h.Dirs.Queued, entries[0].Name())
		require.Equal(t, reserver.recorded.Enqueued.Id, runners.Queued().Id(dir).String())

		// the archive is rewound after being uploaded to the control plane.
		kernel, err := os.ReadFile(filepath.Join(dir, "archive.tar.gz"))
		require.NoError(t, err)
		require.Equal(t, "kernel contents", string(kernel))

		environ, err := os.ReadFile(filepath.Join(dir, eg.EnvironFile))
		require.NoError(t, err)
		require.Equal(t, "environ contents", string(environ))

		require.NoFileExists(t, filepath.Join(dir, "candidate.token"))

		encoded, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
		require.NoError(t, err)

		var md runners.EnqueuedDequeueResponse
		require.NoError(t, json.Unmarshal(encoded, &md))
		require.Equal(t, reserver.recorded.Enqueued.Id, md.Enqueued.Id)
		require.Equal(t, "main.wasm", md.Enqueued.Entry)
		require.Equal(t, "access-token", md.AccessToken)
	})

	t.Run("uploads the control plane refuses to record are rejected without touching the spool", func(t *testing.T) {
		reserver := &fakereserver{err: errors.New("forbidden")}
		h := &daemons.UploadHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Reserver: reserver}

		enqresp := runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String()},
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newUploadRequest(t, &enqresp, "candidate-token"))
		require.Equal(t, http.StatusBadGateway, w.Code)
		require.True(t, reserver.called)

		requireEmptyDir(t, h.Dirs.Queued)
		requireEmptyDir(t, h.Dirs.Downloading)
	})

	t.Run("spool failures after recording the workload are surfaced", func(t *testing.T) {
		reserver := &fakereserver{recorded: newRecorded()}
		h := &daemons.UploadHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Reserver: reserver}

		// a regular file in place of the downloading directory prevents spooling.
		h.Dirs.Downloading = filepath.Join(t.TempDir(), "downloading")
		require.NoError(t, os.WriteFile(h.Dirs.Downloading, nil, 0600))

		enqresp := runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String()},
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newUploadRequest(t, &enqresp, "candidate-token"))
		require.Equal(t, http.StatusInternalServerError, w.Code)

		// the workload was recorded with the control plane before spooling failed.
		require.True(t, reserver.called)
		require.Equal(t, "candidate-token", reserver.token)

		requireEmptyDir(t, h.Dirs.Queued)
	})

	t.Run("uploads that would exceed target load are rejected without touching the spool", func(t *testing.T) {
		reserver := &fakereserver{recorded: newRecorded()}
		h := &daemons.UploadHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Reserver: reserver}
		h.RM = runners.NewResourceManager(runners.RuntimeResources{Cores: 10, Memory: 10, Vram: 10})

		enqresp := runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), Cores: 9},
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newUploadRequest(t, &enqresp, "candidate-token"))
		require.Equal(t, http.StatusConflict, w.Code)
		require.False(t, reserver.called)

		requireEmptyDir(t, h.Dirs.Queued)
		requireEmptyDir(t, h.Dirs.Downloading)
	})

	t.Run("missing kernel file is rejected", func(t *testing.T) {
		reserver := &fakereserver{recorded: newRecorded()}
		h := &daemons.UploadHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Reserver: reserver}

		enqresp := runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String()},
		}
		mimetype, body, err := runners.NewWorkloadRequest(&enqresp, strings.NewReader(""))
		require.NoError(t, err)
		defer body.Close()

		r := httptest.NewRequest(http.MethodPost, "/c/upload", body)
		r.Header.Set("Content-Type", mimetype)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.False(t, reserver.called)

		requireEmptyDir(t, h.Dirs.Queued)
	})

	t.Run("missing candidate token is rejected", func(t *testing.T) {
		reserver := &fakereserver{recorded: newRecorded()}
		h := &daemons.UploadHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Reserver: reserver}

		enqresp := runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String()},
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, newUploadRequest(t, &enqresp, ""))
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.False(t, reserver.called)

		requireEmptyDir(t, h.Dirs.Queued)
	})

	t.Run("missing enqueued metadata is rejected", func(t *testing.T) {
		reserver := &fakereserver{recorded: newRecorded()}
		h := &daemons.UploadHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Reserver: reserver}

		r := httptest.NewRequest(http.MethodPost, "/c/upload", strings.NewReader("not json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.False(t, reserver.called)
	})
}
