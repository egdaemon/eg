package runners

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestPushClientCandidates(t *testing.T) {
	t.Run("requests candidates matching the workload requirements", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/c/q/candidates", r.URL.Path)
			require.Equal(t, "4", r.FormValue("cores"))
			require.Equal(t, "8", r.FormValue("vram"))
			require.Equal(t, "true", r.FormValue("allow_shared"))
			require.Equal(t, "https://example.com/repo.git", r.FormValue("vcs_uri"))
			require.Equal(t, "3", r.FormValue("limit"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"id":"runner-1","p2pid":"peer-1"}]}`))
		}))
		defer srv.Close()

		resp, err := PushClient{c: srv.Client(), host: srv.URL}.Candidates(t.Context(), &computeapi.Enqueued{
			Cores:       4,
			Vram:        8,
			AllowShared: true,
			VcsUri:      "https://example.com/repo.git",
		}, 3)
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		require.Equal(t, "runner-1", resp.Items[0].Id)
		require.Equal(t, "peer-1", resp.Items[0].P2Pid)
	})
}

func TestTokenClaims(t *testing.T) {
	sign := func(t *testing.T, claims jwt.Claims) string {
		t.Helper()
		token, err := jwtx.Signed([]byte("derp"), claims)
		require.NoError(t, err)
		return token
	}

	t.Run("log token", func(t *testing.T) {
		relay, p2pid := tokenClaims(sign(t, struct {
			jwt.RegisteredClaims
			Relay    []string `json:"relay"`
			P2P      string   `json:"p2p"`
			Workload string   `json:"wid"`
		}{Relay: []string{"/ip4/127.0.0.1/tcp/4001/p2p/relay"}, P2P: "peer-1", Workload: "wid"}))
		require.Equal(t, []string{"/ip4/127.0.0.1/tcp/4001/p2p/relay"}, relay)
		require.Equal(t, "peer-1", p2pid)
	})

	t.Run("candidate token", func(t *testing.T) {
		relay, p2pid := tokenClaims(sign(t, struct {
			jwt.RegisteredClaims
			Relay []string `json:"relay"`
		}{Relay: []string{"/ip4/127.0.0.1/tcp/4001/p2p/relay"}}))
		require.Equal(t, []string{"/ip4/127.0.0.1/tcp/4001/p2p/relay"}, relay)
		require.Empty(t, p2pid)
	})

	t.Run("garbage", func(t *testing.T) {
		relay, p2pid := tokenClaims("derp")
		require.Empty(t, relay)
		require.Empty(t, p2pid)
	})
}
