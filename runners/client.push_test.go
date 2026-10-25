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

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
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

		// drain the request like net/http does; closing a stream with unread data resets it.
		_, _ = io.Copy(io.Discard, req.Body)
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
			received computeapi.EnqueuedDequeueResponse
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

			// the runner responds with the workload as recorded by the control plane.
			recorded := proto.Clone(received.Enqueued).(*computeapi.Enqueued)
			recorded.LogsToken = "logs-token"
			w.WriteHeader(http.StatusAccepted)
			require.NoError(t, json.NewEncoder(w).Encode(recorded))
		}))

		candidates := []*compute.Compute{
			{Id: "busy", P2Pid: busy.ID().String(), Token: "busy-token"},
			{Id: "accepting", P2Pid: accepting.ID().String(), Token: "accepting-token"},
		}

		req := &computeapi.EnqueuedDequeueResponse{
			Enqueued: &computeapi.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), AccountId: "acct", Entry: "main.wasm"},
		}

		accepted, recorded := runners.TryUpload(t.Context(), client, candidates, req, strings.NewReader("kernel contents"), strings.NewReader("environ contents"))
		require.NotNil(t, accepted)
		require.Equal(t, "accepting", accepted.Id)
		require.NotNil(t, recorded)
		require.Equal(t, req.Enqueued.Id, recorded.Id)
		require.Equal(t, "logs-token", recorded.LogsToken)
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

		req := &computeapi.EnqueuedDequeueResponse{
			Enqueued: &computeapi.Enqueued{Id: uuid.Must(uuid.NewV7()).String()},
		}

		accepted, recorded := runners.TryUpload(t.Context(), client, candidates, req, strings.NewReader("kernel contents"), strings.NewReader(""))
		require.Nil(t, accepted)
		require.Nil(t, recorded)
	})
}
