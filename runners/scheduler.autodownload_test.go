package runners

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

// recordingstrategy is a backoff.Strategy that always returns a small constant
// delay (so the loop spins quickly) while recording the attempt number it was
// invoked with, allowing tests to observe Reset() calls.
type recordingstrategy struct {
	delay    time.Duration
	attempts []int64
}

func (t *recordingstrategy) Backoff(attempt int64) time.Duration {
	t.attempts = append(t.attempts, attempt)
	return t.delay
}

func dequeueresponse(t *testing.T) string {
	t.Helper()
	uid := errorsx.Must(uuid.NewV4())
	return fmt.Sprintf(`{"enqueued":{"id":%q,"entry":"entry.wasm","cores":1,"memory":1048576}}`, uid.String())
}

func newtestclient(t *testing.T, srv *httptest.Server) *http.Client {
	t.Helper()
	dst := errorsx.Must(url.Parse(srv.URL))
	return &http.Client{Transport: httpx.RewriteHostTransport(dst, nil)}
}

func TestAutoDownload(t *testing.T) {
	newresources := func() *ResourceManager {
		return NewResourceManager(RuntimeResources{Cores: 4, Memory: 4 * 1024 * 1024 * 1024})
	}

	t.Run("404 download errors reset the backoff attempts", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/dequeue") {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, dequeueresponse(t))
				return
			}

			errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusNotFound))
		}))
		defer srv.Close()

		strategy := &recordingstrategy{delay: 7 * time.Millisecond}
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		autodownload(ctx, newtestclient(t, srv), newresources(), strategy, NewSpoolDir(t.TempDir()))

		require.NotEmpty(t, strategy.attempts)
		// truncate the final attempt since it can be non zero due to context cancellation.
		strategy.attempts = strategy.attempts[:len(strategy.attempts)-1]
		for _, attempt := range strategy.attempts {
			require.Equal(t, int64(0), attempt, "404s should reset the backoff attempts")
		}
	})

	t.Run("general download errors grow the backoff attempts", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/dequeue") {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, dequeueresponse(t))
				return
			}

			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		strategy := &recordingstrategy{delay: time.Millisecond}
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		autodownload(ctx, newtestclient(t, srv), newresources(), strategy, NewSpoolDir(t.TempDir()))

		require.GreaterOrEqual(t, len(strategy.attempts), 2)
		for i := 1; i < len(strategy.attempts); i++ {
			require.Equal(t, strategy.attempts[i-1]+1, strategy.attempts[i], "errors without a reset should grow the backoff attempts")
		}
	})
}
