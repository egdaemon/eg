package computeapi

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/stretchr/testify/require"
)

func TestRunnerHistory(t *testing.T) {
	runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
	require.NoError(t, err)
	defer runner.Close()

	runner.SetStreamHandler(proxyProtocol, func(s network.Stream) {
		defer s.Close()

		req, err := http.ReadRequest(bufio.NewReader(s))
		if err != nil {
			return
		}

		rec := httptest.NewRecorder()
		switch req.URL.Path {
		case "/c/history/w1":
			_, _ = rec.WriteString(`{"history":{"result":2}}`)
		case "/c/history/w1/logs":
			_, _ = rec.WriteString("from the runner\n")
		case "/c/history/w1/analytics":
			_, _ = rec.WriteString("recorded analytics")
		case "/c/analytics/w1":
			_, _ = rec.WriteString("live analytics")
		default:
			rec.WriteHeader(http.StatusNotFound)
		}

		_, _ = io.Copy(io.Discard, req.Body)
		_ = rec.Result().Write(s)
	})

	client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
	require.NoError(t, err)
	defer client.Close()
	client.Peerstore().AddAddrs(runner.ID(), runner.Addrs(), peerstore.TempAddrTTL)

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"p2p": runner.ID().String()}).SignedString([]byte("secret"))
	require.NoError(t, err)

	// returning from either must release the stream, closing it used to recurse until the stack overflowed.
	t.Run("find", func(t *testing.T) {
		found, err := RunnerHistoryFind(t.Context(), client, token, "w1")
		require.NoError(t, err)
		require.EqualValues(t, 2, found.Result)
	})

	t.Run("logs", func(t *testing.T) {
		var dst bytes.Buffer
		require.NoError(t, RunnerHistoryLogs(t.Context(), client, token, "w1", &dst))
		require.Equal(t, "from the runner\n", dst.String())
	})

	t.Run("history analytics", func(t *testing.T) {
		var dst bytes.Buffer
		require.NoError(t, RunnerHistoryAnalytics(t.Context(), client, token, "w1", &dst))
		require.Equal(t, "recorded analytics", dst.String())
	})

	t.Run("analytics", func(t *testing.T) {
		var dst bytes.Buffer
		require.NoError(t, RunnerAnalytics(t.Context(), client, token, "w1", &dst))
		require.Equal(t, "live analytics", dst.String())
	})
}
