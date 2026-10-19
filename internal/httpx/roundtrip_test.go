package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

func TestNewFixedStatusClient(t *testing.T) {
	t.Run("returns fixed status for GET", func(t *testing.T) {
		c := httpx.NewFixedStatusClient(http.StatusUnauthorized)
		req, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns fixed status for POST", func(t *testing.T) {
		c := httpx.NewFixedStatusClient(http.StatusServiceUnavailable)
		req, err := http.NewRequest(http.MethodPost, "http://example.com/", strings.NewReader("body"))
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	})

	t.Run("returns same status regardless of URL", func(t *testing.T) {
		c := httpx.NewFixedStatusClient(http.StatusForbidden)
		for _, u := range []string{
			"http://example.com/foo",
			"http://other.example.com/bar",
			"http://example.com/baz?q=1",
		} {
			req, err := http.NewRequest(http.MethodGet, u, nil)
			require.NoError(t, err)
			resp, err := c.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, resp.StatusCode)
		}
	})
}

func TestRewriteHostTransport(t *testing.T) {
	serve := func(t *testing.T, target string) *http.Request {
		t.Helper()

		var seen *http.Request
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r
		}))
		t.Cleanup(srv.Close)

		dst, err := url.Parse(srv.URL)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
		require.NoError(t, err)

		resp, err := (&http.Client{Transport: httpx.RewriteHostTransport(dst, nil)}).Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.NotNil(t, seen)
		return seen
	}

	t.Run("should send the request to the destination host", func(t *testing.T) {
		seen := serve(t, "https://api.example.com/a/b")
		require.Equal(t, "/a/b", seen.URL.Path)
	})

	t.Run("should keep the query", func(t *testing.T) {
		seen := serve(t, "https://api.example.com/a?query=prod&offset=3")
		require.Equal(t, "prod", seen.URL.Query().Get("query"))
		require.Equal(t, "3", seen.URL.Query().Get("offset"))
	})

	t.Run("should keep the escaped path", func(t *testing.T) {
		seen := serve(t, "https://api.example.com/a/b%2Fc")
		require.Equal(t, "/a/b%2Fc", seen.URL.EscapedPath())
		require.Equal(t, "/a/b/c", seen.URL.Path)
	})
}
