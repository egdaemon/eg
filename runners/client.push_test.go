package runners_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// newRunner stands in for a runner's "/egdaemon/proxy" stream handler
// (cmd/eg/daemons/p2p.go), feeding the raw HTTP request off the stream
// straight into the given handler instead of proxying to a local listener.
func newRunner(t *testing.T, client host.Host, handler http.Handler) host.Host {
	t.Helper()

	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })

	h.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
		defer s.Close()

		req, err := http.ReadRequest(bufio.NewReader(s))
		if err != nil {
			return
		}

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		_ = rec.Result().Write(s)
	})

	ctx, done := context.WithTimeout(t.Context(), 5*time.Second)
	defer done()
	require.NoError(t, client.Connect(ctx, peer.AddrInfo{ID: h.ID(), Addrs: h.Addrs()}))

	return h
}

func TestTryUpload(t *testing.T) {
	t.Run("uploads the workload to the first accepting runner", func(t *testing.T) {
		var (
			received runners.EnqueuedDequeueResponse
			token    string
			kernel   string
			environ  string
		)

		client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer client.Close()

		busy := newRunner(t, client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
		}))

		accepting := newRunner(t, client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/c/upload", r.URL.Path)
			require.NoError(t, json.Unmarshal([]byte(r.FormValue("enqueued")), &received))
			token = r.FormValue("token")

			f, _, err := r.FormFile("kernel")
			require.NoError(t, err)
			defer f.Close()
			b, err := io.ReadAll(f)
			require.NoError(t, err)
			kernel = string(b)

			e, _, err := r.FormFile("environ")
			require.NoError(t, err)
			defer e.Close()
			b, err = io.ReadAll(e)
			require.NoError(t, err)
			environ = string(b)

			w.WriteHeader(http.StatusAccepted)
		}))

		candidates := []*compute.Compute{
			{Id: "busy", P2Pid: busy.ID().String(), Token: "busy-token"},
			{Id: "accepting", P2Pid: accepting.ID().String(), Token: "accepting-token"},
		}

		req := &runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), AccountId: "acct", Entry: "main.wasm"},
		}

		accepted := runners.TryUpload(t.Context(), client, candidates, req, strings.NewReader("kernel contents"), strings.NewReader("environ contents"))
		require.NotNil(t, accepted)
		require.Equal(t, "accepting", accepted.Id)
		require.Equal(t, req.Enqueued.Id, received.Enqueued.Id)
		require.Equal(t, "acct", received.Enqueued.AccountId)
		require.Equal(t, "accepting-token", token)
		require.Equal(t, "kernel contents", kernel)
		require.Equal(t, "environ contents", environ)
	})

	t.Run("returns nil when no runner accepts the workload", func(t *testing.T) {
		client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer client.Close()

		disabled := newRunner(t, client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))

		unreachable, err := libp2p.New(libp2p.NoListenAddrs)
		require.NoError(t, err)
		require.NoError(t, unreachable.Close())

		candidates := []*compute.Compute{
			{Id: "disabled", P2Pid: disabled.ID().String()},
			{Id: "unreachable", P2Pid: unreachable.ID().String()},
			{Id: "invalid", P2Pid: "not a peer id"},
		}

		req := &runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String()},
		}

		require.Nil(t, runners.TryUpload(t.Context(), client, candidates, req, strings.NewReader("kernel contents"), strings.NewReader("")))
	})
}
