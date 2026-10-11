package daemons_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func logsecret() []byte {
	return []byte("logs test secret")
}

func newLogsServer(t *testing.T, dirs runners.SpoolDirs) *httptest.Server {
	t.Helper()
	r := mux.NewRouter()
	r.Handle("/c/logs/{id}", &daemons.LogsHandler{Dirs: dirs, Secret: logsecret}).Methods(http.MethodGet)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func logsRequest(t *testing.T, srv *httptest.Server, id string, token string, query string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/c/logs/"+id+query, nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func logsToken(t *testing.T, wid string) string {
	t.Helper()
	token, err := runners.SignLogToken(logsecret, wid, time.Minute)
	require.NoError(t, err)
	return token
}

func runningLog(t *testing.T, dirs runners.SpoolDirs, uid uuid.UUID) *os.File {
	t.Helper()
	runtimedir := dirs.Path(uid).Resolve("work", eg.RuntimeDirectory)
	require.NoError(t, os.MkdirAll(runtimedir, 0700))
	logs, err := os.Create(filepath.Join(runtimedir, runners.DaemonLogName))
	require.NoError(t, err)
	t.Cleanup(func() { _ = logs.Close() })
	return logs
}

func TestLogsHandler(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		srv := newLogsServer(t, runners.NewSpoolDir(t.TempDir()))
		resp := logsRequest(t, srv, uuid.Must(uuid.NewV7()).String(), "", "")
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("token signed with another secret", func(t *testing.T) {
		srv := newLogsServer(t, runners.NewSpoolDir(t.TempDir()))
		uid := uuid.Must(uuid.NewV7())
		token, err := runners.SignLogToken(func() []byte { return []byte("derp") }, uid.String(), time.Minute)
		require.NoError(t, err)
		resp := logsRequest(t, srv, uid.String(), token, "")
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("expired token", func(t *testing.T) {
		srv := newLogsServer(t, runners.NewSpoolDir(t.TempDir()))
		uid := uuid.Must(uuid.NewV7())
		resp := logsRequest(t, srv, uid.String(), mustExpired(t, uid.String()), "")
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("token for another workload", func(t *testing.T) {
		srv := newLogsServer(t, runners.NewSpoolDir(t.TempDir()))
		resp := logsRequest(t, srv, uuid.Must(uuid.NewV7()).String(), logsToken(t, uuid.Must(uuid.NewV7()).String()), "")
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("invalid id", func(t *testing.T) {
		srv := newLogsServer(t, runners.NewSpoolDir(t.TempDir()))
		resp := logsRequest(t, srv, "derp", logsToken(t, "derp"), "")
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("invalid offset", func(t *testing.T) {
		dirs := runners.NewSpoolDir(t.TempDir())
		srv := newLogsServer(t, dirs)
		uid := uuid.Must(uuid.NewV7())
		runningLog(t, dirs, uid)
		resp := logsRequest(t, srv, uid.String(), logsToken(t, uid.String()), "?offset=-1")
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("unknown workload", func(t *testing.T) {
		srv := newLogsServer(t, runners.NewSpoolDir(t.TempDir()))
		uid := uuid.Must(uuid.NewV7())
		resp := logsRequest(t, srv, uid.String(), logsToken(t, uid.String()), "")
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("streams until the workload completes", func(t *testing.T) {
		dirs := runners.NewSpoolDir(t.TempDir())
		srv := newLogsServer(t, dirs)
		uid := uuid.Must(uuid.NewV7())
		logs := runningLog(t, dirs, uid)
		_, err := logs.WriteString("skipped hello\n")
		require.NoError(t, err)

		resp := logsRequest(t, srv, uid.String(), logsToken(t, uid.String()), "?offset=8")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

		buf := make([]byte, len("hello\n"))
		_, err = io.ReadFull(resp.Body, buf)
		require.NoError(t, err)
		require.Equal(t, "hello\n", string(buf))

		_, err = logs.WriteString("world\n")
		require.NoError(t, err)
		_, err = io.ReadFull(resp.Body, buf)
		require.NoError(t, err)
		require.Equal(t, "world\n", string(buf))

		_, err = logs.WriteString("done\n")
		require.NoError(t, err)
		require.NoError(t, os.RemoveAll(dirs.Path(uid).Resolve()))

		remaining, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "done\n", string(remaining))
	})
}

func mustExpired(t *testing.T, wid string) string {
	t.Helper()
	token, err := runners.SignLogToken(logsecret, wid, -time.Minute)
	require.NoError(t, err)
	return token
}
