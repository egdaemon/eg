package cmdme

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	profiles := []*authn.Authn{
		{Account: &authn.Account{Id: "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b", Description: "acme"}, Profile: &authn.Profile{Id: "p1", AccountId: "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b", Display: "alice", Email: "alice@acme.test"}},
		{Account: &authn.Account{Id: "1190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"}, Profile: &authn.Profile{Id: "p2", AccountId: "1190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b", Display: "alice2", Email: "alice@other.test"}},
	}

	t.Run("should show the current profile before the table with the current row marked", func(t *testing.T) {
		out := ansi.Strip(render("0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b", profiles...))

		require.Regexp(t, `^current: 0190a1b2 - acme - alice\n`, out)
		require.Regexp(t, `\* +│ 0190a1b2 - acme +│ alice +│ alice@acme.test`, out)
		require.Regexp(t, `  +│ 1190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b +│ alice2 +│ alice@other.test`, out)
	})

	t.Run("should list every profile when the current account is unknown", func(t *testing.T) {
		out := ansi.Strip(render("", profiles...))

		require.Contains(t, out, "current: unknown")
		require.Contains(t, out, "alice@acme.test")
		require.Contains(t, out, "alice@other.test")
		require.NotContains(t, out, "*")
	})
}

func TestEditRun(t *testing.T) {
	t.Setenv("EG_SESSION_TOKEN", "stub-refresh-token")

	var (
		m             sync.Mutex
		authorization string
		method, path  string
		body          string
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"access_token":"stub-access-token","token_type":"bearer","expires_in":3600}`)
	})
	mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"profiles":[{"token":"the-token","profile":{"id":"profile-1","account_id":"account-1","display":"old","email":"old@example.com"}}]}`)
	})
	mux.HandleFunc("/authn/current", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"token":"stub-session-token","profile":{"id":"profile-1"},"account":{"id":"account-1"}}`)
	})
	mux.HandleFunc("/m/authz/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"token":{"iss":"account-1","bearer":"meta-bearer","expires":%d}}`, time.Now().Add(time.Hour).UnixMilli())
	})
	mux.HandleFunc("/me/", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		m.Lock()
		defer m.Unlock()
		authorization, method, path, body = r.Header.Get("Authorization"), r.Method, r.URL.Path, string(b)
		fmt.Fprint(w, `{"profile":{"id":"profile-1","account_id":"account-1","display":"alice","email":"alice@example.com"}}`)
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
	ident := cmdidentity.New(t.Context(), "account-1", nil)

	require.NoError(t, Edit{SSHKeyPath: filepath.Join(t.TempDir(), "sshkey"), Display: "alice"}.Run(gctx, tlsc, ident))

	m.Lock()
	defer m.Unlock()
	require.Equal(t, "Bearer meta-bearer", authorization)
	require.Equal(t, http.MethodPatch, method)
	require.Equal(t, "/me/", path)
	// the email was not specified, it keeps its current value.
	require.JSONEq(t, `{"profile":{"display":"alice","email":"old@example.com"}}`, body)
}
