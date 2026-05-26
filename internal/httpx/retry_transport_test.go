package httpx_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/httptestx"
	. "github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

func TestRetryTransport(t *testing.T) {
	t.Run("should retry once", func(t *testing.T) {
		invoked := 0
		body := []byte("")
		c := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			body, _ = io.ReadAll(req.Body)
			invoked++

			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}
		})

		c.Transport = NewRetryTransport(c.Transport, http.StatusBadGateway)
		req, err := http.NewRequest(http.MethodGet, "http://example.com/", strings.NewReader("Hello World"))
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadGateway, resp.StatusCode)
		require.Equal(t, 2, invoked)
		require.Equal(t, []byte("Hello World"), body)
	})

	t.Run("should retry on context.DeadlineExceeded", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			time.Sleep(time.Hour)
		}))
		defer s.Close()
		c := &http.Client{}
		c.Transport = NewRetryTransport(c.Transport, http.StatusBadGateway)

		ctx, done := context.WithTimeout(context.Background(), -1*time.Second)
		defer done()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, strings.NewReader("Hello World"))
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("should retry with a nil body", func(t *testing.T) {
		invoked := 0
		body := []byte("")
		c := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			body, _ = io.ReadAll(req.Body)
			invoked++

			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}
		})

		c.Transport = NewRetryTransport(c.Transport, http.StatusBadGateway)
		req, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadGateway, resp.StatusCode)
		require.Equal(t, 2, invoked)
		require.Equal(t, []byte(""), body)
	})
}
