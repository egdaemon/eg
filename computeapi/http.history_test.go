package computeapi_test

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func newHistoryServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
	t.Cleanup(func() { _ = db.Close() })
	r := mux.NewRouter()
	computeapi.NewHTTPHistory(db, logsecret).Bind(r.PathPrefix("/c/history").Subrouter())
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, db
}

func historyRequest(t *testing.T, srv *httptest.Server, path string, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestHistoryHandler(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		srv, _ := newHistoryServer(t)
		resp := historyRequest(t, srv, "/c/history/"+uuid.Must(uuid.NewV7()).String(), "")
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("token for another workload", func(t *testing.T) {
		srv, _ := newHistoryServer(t)
		resp := historyRequest(t, srv, "/c/history/"+uuid.Must(uuid.NewV7()).String(), logsToken(t, uuid.Must(uuid.NewV7()).String()))
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("not recorded", func(t *testing.T) {
		srv, _ := newHistoryServer(t)
		id := uuid.Must(uuid.NewV7()).String()
		require.Equal(t, http.StatusNotFound, historyRequest(t, srv, "/c/history/"+id, logsToken(t, id)).StatusCode)
		require.Equal(t, http.StatusNotFound, historyRequest(t, srv, "/c/history/"+id+"/logs", logsToken(t, id)).StatusCode)
	})

	t.Run("serves the record and logs", func(t *testing.T) {
		srv, db := newHistoryServer(t)
		id := uuid.Must(uuid.NewV7()).String()
		logs := filepath.Join(t.TempDir(), "w.log")
		require.NoError(t, os.WriteFile(logs, []byte("output"), 0600))
		require.NoError(t, computex.EntryUpsert(t.Context(), db, langx.Clone(computex.Entry{}, computex.EntryOptionTestDefaults, computex.EntryOptionID(id), func(e *computex.Entry) {
			e.Result = computex.ResultFailure
			e.LogsPath = logs
		})).Scan(&computex.Entry{}))

		resp := historyRequest(t, srv, "/c/history/"+id, logsToken(t, id))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var out compute.HistoryFindResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
		require.Equal(t, id, out.History.Id)
		require.EqualValues(t, computex.ResultFailure, out.History.Result)

		resp = historyRequest(t, srv, "/c/history/"+id+"/logs", logsToken(t, id))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "output", string(b))
	})

	t.Run("serves the recorded analytics", func(t *testing.T) {
		srv, db := newHistoryServer(t)
		id := uuid.Must(uuid.NewV7()).String()
		analytics := filepath.Join(t.TempDir(), "w.analytics.db")
		require.NoError(t, os.WriteFile(analytics, []byte("analytics"), 0600))
		require.NoError(t, computex.EntryUpsert(t.Context(), db, langx.Clone(computex.Entry{}, computex.EntryOptionTestDefaults, computex.EntryOptionID(id), func(e *computex.Entry) {
			e.AnalyticsPath = analytics
		})).Scan(&computex.Entry{}))

		resp := historyRequest(t, srv, "/c/history/"+id+"/analytics", logsToken(t, id))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "analytics", string(b))
	})

	t.Run("no analytics recorded", func(t *testing.T) {
		srv, db := newHistoryServer(t)
		id := uuid.Must(uuid.NewV7()).String()
		require.NoError(t, computex.EntryUpsert(t.Context(), db, langx.Clone(computex.Entry{}, computex.EntryOptionTestDefaults, computex.EntryOptionID(id))).Scan(&computex.Entry{}))

		require.Equal(t, http.StatusNotFound, historyRequest(t, srv, "/c/history/"+id+"/analytics", logsToken(t, id)).StatusCode)
	})
}

func logsecret() []byte {
	return []byte("history test secret")
}

func logsToken(t *testing.T, wid string) string {
	t.Helper()
	token, err := computeapi.SignLogToken(logsecret, wid, time.Minute)
	require.NoError(t, err)
	return token
}
