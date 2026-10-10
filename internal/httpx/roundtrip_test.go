package httpx_test

import (
	"net/http"
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
