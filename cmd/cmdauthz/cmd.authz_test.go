package cmdauthz

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/stretchr/testify/require"
)

// FakeAPI serves the ssh identity exchange and both authorization endpoints, every host is
// rewritten to it. the ci tokens are ci-bearer, the meta tokens meta-bearer.
func fakeapi(t *testing.T) (*cmdopts.Global, *cmdopts.TLSConfig, *cmdidentity.T) {
	t.Setenv(userx.EnvCacheDirectory, t.TempDir())
	t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
	t.Setenv("EG_SESSION_TOKEN", "stub-refresh-token")

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"access_token":"stub-access-token","token_type":"bearer","expires_in":3600}`)
	})
	mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"profiles":[{"token":"the-token","profile":{"id":"profile-1","account_id":"account-1"}}]}`)
	})
	mux.HandleFunc("/authn/current", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"token":"stub-session-token","profile":{"id":"profile-1"},"account":{"id":"account-1"}}`)
	})
	mux.HandleFunc("/m/authz/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"token":{"iss":"account-1","bearer":"meta-bearer","expires":%d}}`, time.Now().Add(time.Hour).UnixMilli())
	})
	mux.HandleFunc("/c/authz/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"token":{"iss":"account-1","bearer":"ci-bearer","expires":%d}}`, time.Now().Add(time.Hour).UnixMilli())
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	dst := errorsx.Must(url.Parse(srv.URL))
	tlsc := &cmdopts.TLSConfig{Wrap: func(client *http.Client) *http.Client {
		client.Transport = httpx.RewriteHostTransport(dst, client.Transport)
		return client
	}}

	return &cmdopts.Global{Context: t.Context(), AccountID: "account-1"}, tlsc, cmdidentity.New(t.Context(), "account-1", "", nil)
}

func TestToken(t *testing.T) {
	gctx, tlsc, ident := fakeapi(t)
	id := Identity{SSHKeyPath: filepath.Join(t.TempDir(), "sshkey")}

	t.Run("should print the ci token", func(t *testing.T) {
		var out bytes.Buffer
		require.NoError(t, id.print(&out, gctx, tlsc, ident, authn.EndpointCompute()))
		require.Equal(t, "ci-bearer\n", out.String())
	})

	t.Run("should print the meta token", func(t *testing.T) {
		var out bytes.Buffer
		require.NoError(t, id.print(&out, gctx, tlsc, ident, authn.EndpointMetaAuthz()))
		require.Equal(t, "meta-bearer\n", out.String())
	})
}
