package cmdgcp

import (
	"fmt"
	"github.com/egdaemon/eg/internal/userx"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

// every wif command resolves the account through the injected identity, an explicit account needs no lookup,
// and then talks to the wif routes with the meta token.
func TestRun(t *testing.T) {
	// the identities are remembered in the cache, keep it out of the user's.
	t.Setenv(userx.EnvCacheDirectory, t.TempDir())
	t.Setenv("RUNTIME_DIRECTORY", t.TempDir())

	cases := []struct {
		name   string
		run    func(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, sshkey string) error
		method string
		path   string
		body   string
	}{
		{
			name: "create",
			run: func(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, sshkey string) error {
				return Create{Selector: Selector{SSHKeyPath: sshkey}, Audience: "aud"}.Run(gctx, tlsc, ident)
			},
			method: http.MethodPost,
			path:   "/m/gcp/wif/",
			body:   `{"connection":{"label":"account-1","audience":"aud"}}`,
		},
		{
			name: "list",
			run: func(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, sshkey string) error {
				return List{SSHKeyPath: sshkey}.Run(gctx, tlsc, ident)
			},
			method: http.MethodGet,
			path:   "/m/gcp/wif/",
			body:   `{"items":[{"label":"account-1","audience":"aud"}]}`,
		},
		{
			name: "edit",
			run: func(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, sshkey string) error {
				return Edit{Selector: Selector{SSHKeyPath: sshkey}, Audience: "aud"}.Run(gctx, tlsc, ident)
			},
			method: http.MethodPatch,
			path:   "/m/gcp/wif/account-1",
			body:   `{"connection":{"label":"account-1","audience":"aud"}}`,
		},
		{
			name: "remove",
			run: func(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, sshkey string) error {
				return Remove{Selector: Selector{SSHKeyPath: sshkey}}.Run(gctx, tlsc, ident)
			},
			method: http.MethodDelete,
			path:   "/m/gcp/wif/account-1",
			body:   `{}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("EG_SESSION_TOKEN", "stub-refresh-token")

			var (
				m             sync.Mutex
				authorization string
				method, path  string
			)

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
				fmt.Fprintf(w, `{"token":{"iss":"account-1","bearer":"meta-bearer","expires":%d,"usermanagement":true}}`, time.Now().Add(time.Hour).UnixMilli())
			})
			mux.HandleFunc("/m/gcp/wif/", func(w http.ResponseWriter, r *http.Request) {
				m.Lock()
				defer m.Unlock()
				authorization, method, path = r.Header.Get("Authorization"), r.Method, r.URL.Path
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, c.body)
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
			gctx := &cmdopts.Global{Context: t.Context(), AccountID: "account-1"}
			ident := cmdidentity.New(t.Context(), "account-1", "", nil)

			require.NoError(t, c.run(gctx, tlsc, ident, filepath.Join(t.TempDir(), "sshkey")))

			m.Lock()
			defer m.Unlock()
			require.Equal(t, "Bearer meta-bearer", authorization)
			require.Equal(t, c.method, method)
			require.Equal(t, c.path, path)
		})
	}
}
