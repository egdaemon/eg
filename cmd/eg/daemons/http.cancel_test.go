package daemons_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func newCancelServer(t *testing.T, dirs runners.SpoolDirs) *httptest.Server {
	t.Helper()
	r := mux.NewRouter()
	r.Handle("/c/cancel/{id}", &daemons.CancelHandler{Dirs: dirs, Secret: logsecret}).Methods(http.MethodPost)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func cancelRequest(t *testing.T, srv *httptest.Server, id string, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/c/cancel/"+id, nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestCancelHandler(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		srv := newCancelServer(t, runners.NewSpoolDir(t.TempDir()))
		resp := cancelRequest(t, srv, uuid.Must(uuid.NewV7()).String(), "")
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("token for another workload", func(t *testing.T) {
		srv := newCancelServer(t, runners.NewSpoolDir(t.TempDir()))
		resp := cancelRequest(t, srv, uuid.Must(uuid.NewV7()).String(), logsToken(t, uuid.Must(uuid.NewV7()).String()))
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("unknown workload", func(t *testing.T) {
		srv := newCancelServer(t, runners.NewSpoolDir(t.TempDir()))
		uid := uuid.Must(uuid.NewV7())
		resp := cancelRequest(t, srv, uid.String(), logsToken(t, uid.String()))
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("marks the workload and is idempotent", func(t *testing.T) {
		dirs := runners.NewSpoolDir(t.TempDir())
		uid := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid))

		srv := newCancelServer(t, dirs)
		for range 2 {
			resp := cancelRequest(t, srv, uid.String(), logsToken(t, uid.String()))
			require.Equal(t, http.StatusAccepted, resp.StatusCode)
		}

		dir, ok := dirs.Locate(uid)
		require.True(t, ok)
		require.True(t, runners.IsCancelled(dir))
	})
}
