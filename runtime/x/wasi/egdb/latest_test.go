package egdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egdb"
	"github.com/stretchr/testify/require"
)

func TestLatest(t *testing.T) {
	t.Setenv(eg.EnvComputeCacheDirectory, t.TempDir())
	t.Setenv(eg.EnvComputeVCS, "https://example.com/repo.git")
	t.Setenv("EG_GIT_AUTH_ACCESS_TOKEN", "token")

	var searches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "BEARER token", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/c/history/":
			searches++
			require.Equal(t, "success", r.URL.Query().Get("result"))
			require.Equal(t, "main.wasm", r.URL.Query().Get("workload"))
			_, _ = w.Write([]byte(`{"items":[{"id":"abc"}]}`))
		case "/c/history/abc/analytics":
			_, _ = w.Write([]byte("analytics"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	path, err := egdb.Latest(context.Background(), egdb.WithEndpoint(srv.URL), egdb.WithWorkload("main.wasm"))
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "analytics", string(content))

	// second call is served from the cache.
	again, err := egdb.Latest(context.Background(), egdb.WithEndpoint(srv.URL), egdb.WithWorkload("main.wasm"))
	require.NoError(t, err)
	require.Equal(t, path, again)
	require.Equal(t, 1, searches)
}

func TestLatestNoSuccessfulRuns(t *testing.T) {
	t.Setenv(eg.EnvComputeCacheDirectory, t.TempDir())
	t.Setenv("EG_GIT_AUTH_ACCESS_TOKEN", "token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	_, err := egdb.Latest(context.Background(), egdb.WithEndpoint(srv.URL))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestLatestAnalyticsNotStored(t *testing.T) {
	t.Setenv(eg.EnvComputeCacheDirectory, t.TempDir())
	t.Setenv("EG_GIT_AUTH_ACCESS_TOKEN", "token")

	// free accounts do not have their analytics stored.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/c/history/":
			_, _ = w.Write([]byte(`{"items":[{"id":"abc"}]}`))
		default:
			w.WriteHeader(http.StatusPaymentRequired)
		}
	}))
	defer srv.Close()

	_, err := egdb.Latest(context.Background(), egdb.WithEndpoint(srv.URL))
	require.ErrorIs(t, err, os.ErrNotExist)
}
