package daemons_test

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsHandler(t *testing.T) {
	secret := func() []byte { return []byte("analytics test secret") }

	t.Run("missing token", func(t *testing.T) {
		r := mux.NewRouter()
		r.Handle("/c/analytics/{id}", &daemons.AnalyticsHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Secret: secret})
		srv := httptest.NewServer(r)
		defer srv.Close()

		resp, err := srv.Client().Get(srv.URL + "/c/analytics/" + uuid.Must(uuid.NewV7()).String())
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("token for another workload", func(t *testing.T) {
		r := mux.NewRouter()
		r.Handle("/c/analytics/{id}", &daemons.AnalyticsHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Secret: secret})
		srv := httptest.NewServer(r)
		defer srv.Close()

		token, err := computeapi.SignLogToken(secret, uuid.Must(uuid.NewV7()).String(), time.Minute)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/c/analytics/"+uuid.Must(uuid.NewV7()).String(), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", token)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("workload not running", func(t *testing.T) {
		r := mux.NewRouter()
		r.Handle("/c/analytics/{id}", &daemons.AnalyticsHandler{Dirs: runners.NewSpoolDir(t.TempDir()), Secret: secret})
		srv := httptest.NewServer(r)
		defer srv.Close()

		uid := uuid.Must(uuid.NewV7())
		token, err := computeapi.SignLogToken(secret, uid.String(), time.Minute)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/c/analytics/"+uid.String(), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", token)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("running workload without an analytics socket", func(t *testing.T) {
		dirs := runners.NewSpoolDir(t.TempDir())
		uid := uuid.Must(uuid.NewV7())
		require.NoError(t, os.MkdirAll(dirs.Path(uid).Resolve("work", eg.RuntimeDirectory), 0700))

		r := mux.NewRouter()
		r.Handle("/c/analytics/{id}", &daemons.AnalyticsHandler{Dirs: dirs, Secret: secret})
		srv := httptest.NewServer(r)
		defer srv.Close()

		token, err := computeapi.SignLogToken(secret, uid.String(), time.Minute)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/c/analytics/"+uid.String(), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", token)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("serves a snapshot of the live database", func(t *testing.T) {
		// unix socket paths are limited to ~108 bytes, t.TempDir() embeds the test name.
		root, err := os.MkdirTemp("", "an")
		require.NoError(t, err)
		defer os.RemoveAll(root)
		dirs := runners.NewSpoolDir(root)
		uid := uuid.Must(uuid.NewV7())
		runtimedir := dirs.Path(uid).Resolve("work", eg.RuntimeDirectory)
		require.NoError(t, os.MkdirAll(runtimedir, 0700))

		// the runner process holds the database open and serves it over its socket.
		live, err := sql.Open("duckdb", filepath.Join(runtimedir, runners.DaemonAnalyticsName))
		require.NoError(t, err)
		defer live.Close()
		_, err = live.ExecContext(t.Context(), "CREATE TABLE metrics (name TEXT NOT NULL)")
		require.NoError(t, err)
		_, err = live.ExecContext(t.Context(), "INSERT INTO metrics VALUES ('hello')")
		require.NoError(t, err)

		sctx, cancel := context.WithCancel(t.Context())
		errc := make(chan error, 1)
		go func() {
			errc <- duckproxyserver.ListenUnix(sctx, filepath.Join(runtimedir, eg.SocketAnalytics), duckproxyserver.New(live))
		}()
		defer func() {
			cancel()
			<-errc
		}()
		require.Eventually(t, func() bool {
			_, err := os.Stat(filepath.Join(runtimedir, eg.SocketAnalytics))
			return err == nil
		}, 5*time.Second, 10*time.Millisecond)

		r := mux.NewRouter()
		r.Handle("/c/analytics/{id}", &daemons.AnalyticsHandler{Dirs: dirs, Secret: secret})
		srv := httptest.NewServer(r)
		defer srv.Close()

		token, err := computeapi.SignLogToken(secret, uid.String(), time.Minute)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/c/analytics/"+uid.String(), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", token)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		snapshot := filepath.Join(t.TempDir(), "snapshot.db")
		out, err := os.Create(snapshot)
		require.NoError(t, err)
		_, err = io.Copy(out, resp.Body)
		require.NoError(t, err)
		require.NoError(t, out.Close())

		check, err := sql.Open("duckdb", snapshot)
		require.NoError(t, err)
		defer check.Close()
		var name string
		require.NoError(t, check.QueryRowContext(t.Context(), "SELECT name FROM metrics").Scan(&name))
		require.Equal(t, "hello", name)

		entries, err := os.ReadDir(runtimedir)
		require.NoError(t, err)
		for _, e := range entries {
			require.NotContains(t, e.Name(), "snapshot.", "the staged snapshot is removed")
		}
	})
}
