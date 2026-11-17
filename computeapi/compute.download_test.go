package computeapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

// rewrites every request to the test server, the api host is a constant.
type redirectTransport struct {
	target *url.URL
}

func (t redirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme = t.target.Scheme
	r.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(r)
}

func TestDownloadAnalyticsFallbackToRunner(t *testing.T) {
	completed := `{"enqueued":{"id":"w1","completed_at":"` + time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano) + `"}}`

	t.Run("consults the history for the runner before using the stored analytics", func(t *testing.T) {
		var order []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, r.URL.Path)
			switch r.URL.Path {
			case "/c/history/w1":
				// no runner recorded, so the runner can't be used.
				_, _ = w.Write([]byte(`{"history":{"id":"w1","result":2}}`))
			case "/c/q/w1":
				_, _ = w.Write([]byte(completed))
			case "/c/history/w1/analytics":
				_, _ = w.Write([]byte("archived"))
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer srv.Close()

		target, err := url.Parse(srv.URL)
		require.NoError(t, err)

		var dst bytes.Buffer
		require.NoError(t, DownloadAnalytics(t.Context(), &http.Client{Transport: redirectTransport{target: target}}, "w1", &dst))
		require.Equal(t, "archived", dst.String())
		require.Equal(t, []string{"/c/q/w1", "/c/history/w1", "/c/history/w1/analytics"}, order)
	})

	t.Run("stored analytics missing and the history has no runner", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/c/q/w1":
				_, _ = w.Write([]byte(completed))
			case "/c/history/w1/analytics":
				w.WriteHeader(http.StatusNotFound)
			case "/c/history/w1":
				_, _ = w.Write([]byte(`{"history":{"id":"w1","result":2}}`))
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer srv.Close()

		target, err := url.Parse(srv.URL)
		require.NoError(t, err)

		var dst bytes.Buffer
		err = DownloadAnalytics(t.Context(), &http.Client{Transport: redirectTransport{target: target}}, "w1", &dst)
		require.ErrorContains(t, err, "no runner to retrieve from")
		require.NotNil(t, httpx.IsStatusError(err, http.StatusNotFound))
		require.Empty(t, dst.Bytes())
	})

	t.Run("stored analytics missing and the history record is missing", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/c/q/w1" {
				_, _ = w.Write([]byte(completed))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		target, err := url.Parse(srv.URL)
		require.NoError(t, err)

		var dst bytes.Buffer
		err = DownloadAnalytics(t.Context(), &http.Client{Transport: redirectTransport{target: target}}, "w1", &dst)
		require.Error(t, err)
		require.NotNil(t, httpx.IsStatusError(err, http.StatusNotFound))
	})
}
