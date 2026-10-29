package computeapi

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/httpx"
	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v4"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
)

func TestUploadWatch(t *testing.T) {
	enq := &Enqueued{Id: uuid.Must(uuid.NewV7()).String(), AccountId: "acct", Entry: "main.wasm"}

	for _, c := range []struct {
		name     string
		resets   int32 // streams the runner resets before serving the logs.
		attempts int
		backoffs []int64
		logs     []string // control plane requests for the stored log.
	}{
		{name: "follows the logs once the runner can be reached", resets: 2, attempts: 5, backoffs: []int64{0, 1}},
		{name: "falls back to the result when the runner stays unreachable", resets: 100, attempts: 2, backoffs: []int64{0, 1}, logs: []string{"/c/history/w1/logs"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var (
				uploads, streams atomic.Int32
				m                sync.Mutex
				seen             []string
			)

			runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
			require.NoError(t, err)
			defer runner.Close()

			runner.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
				defer s.Close()

				if streams.Add(1) <= c.resets {
					_ = s.Reset()
					return
				}

				req, err := http.ReadRequest(bufio.NewReader(s))
				if err != nil {
					return
				}
				_, _ = io.Copy(io.Discard, req.Body)

				rec := httptest.NewRecorder()
				_, _ = rec.WriteString("from the runner\n")
				_ = rec.Result().Write(s)
			})

			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"p2p": runner.ID().String()}).SignedString([]byte("secret"))
			require.NoError(t, err)

			// the runner doubles as the bootstrap peer the client joins the p2p network through.
			bootstrap := fmt.Sprintf("%s/p2p/%s", runner.Addrs()[0], runner.ID())

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				m.Lock()
				seen = append(seen, r.URL.Path)
				m.Unlock()

				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/c/q/upload":
					uploads.Add(1)
					_, _ = w.Write([]byte(`{"enqueue":{"id":"w1"}}`))
				case "/c/q/w1":
					_, _ = fmt.Fprintf(w, `{"enqueue":{"id":"w1","logs_token":%q,"completed_at":"9999-12-31T23:59:59Z"}}`, token)
				case "/c/runners/meta":
					_, _ = fmt.Fprintf(w, `{"bootstrap":[%q]}`, bootstrap)
				case "/c/history/w1/logs":
					_, _ = w.Write([]byte("stored\n"))
				case "/c/history/w1":
					_, _ = w.Write([]byte(`{"history":{"result":2}}`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()
			dst, err := url.Parse(srv.URL)
			require.NoError(t, err)

			delays := &recordingbackoff{delay: time.Millisecond}
			err = Upload(
				t.Context(),
				&http.Client{Transport: httpx.RewriteHostTransport(dst, nil)},
				nil,
				enq,
				strings.NewReader("kernel"),
				strings.NewReader("environ"),
				UploadEndpoint(srv.URL+"/c/q/upload"),
				UploadPriorities(0, 0),
				UploadFollow(true),
				UploadBackoff(delays),
				UploadMaxAttempts(c.attempts),
			)
			require.NoError(t, err)
			require.EqualValues(t, 1, uploads.Load(), "the accepted workload is never uploaded again")
			require.Equal(t, c.backoffs, delays.attempts)

			m.Lock()
			defer m.Unlock()
			var logs []string
			for _, p := range seen {
				if p == "/c/history/w1/logs" {
					logs = append(logs, p)
				}
			}
			require.Equal(t, c.logs, logs)
		})
	}
}
