package historyapi_test

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

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/history"
	"github.com/egdaemon/eg/internal/historyapi"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func newHistoryServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
	t.Cleanup(func() { _ = db.Close() })
	r := mux.NewRouter()
	historyapi.NewHTTPHistory(db, logsecret).Bind(r.PathPrefix("/c/history").Subrouter())
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
		require.NoError(t, history.EntryUpsert(t.Context(), db, langx.Clone(history.Entry{}, history.EntryOptionTestDefaults, history.EntryOptionID(id), func(e *history.Entry) {
			e.Result = history.ResultFailure
			e.LogsPath = logs
		})).Scan(&history.Entry{}))

		resp := historyRequest(t, srv, "/c/history/"+id, logsToken(t, id))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var out compute.HistoryFindResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
		require.Equal(t, id, out.History.Id)
		require.EqualValues(t, history.ResultFailure, out.History.Result)

		resp = historyRequest(t, srv, "/c/history/"+id+"/logs", logsToken(t, id))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "output", string(b))
	})
}

func logsecret() []byte {
	return []byte("history test secret")
}

func logsToken(t *testing.T, wid string) string {
	t.Helper()
	token, err := runners.SignLogToken(logsecret, wid, time.Minute)
	require.NoError(t, err)
	return token
}
