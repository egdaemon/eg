package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/justinas/alice"
	"github.com/stretchr/testify/require"
)

func TestRollingWriteTimeout(t *testing.T) {
	ctx := t.Context()

	errc := make(chan error, 1)
	s := httptest.NewServer(alice.New(
		httpx.TimeoutRollingWrite(10 * time.Millisecond),
	).ThenFunc(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		time.Sleep(50 * time.Millisecond)
		if _, cause := resp.Write([]byte("hello world")); cause != nil {
			errc <- cause
		}
	})))
	defer s.Close()

	c := &http.Client{}
	c.Transport = httpx.NewRetryTransport(c.Transport, http.StatusBadGateway)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, strings.NewReader("Hello World"))
	require.NoError(t, err)

	resp, err := c.Do(req)
	defer func() {
		errorsx.Log(resp.Body.Close())
	}()
	require.NoError(t, httpx.TryClose(resp))
	require.NoError(t, err)
	require.Error(t, <-errc)
}
