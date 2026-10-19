package cmdgcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/gcpwif"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/stretchr/testify/require"
)

// fake api serving the ssh login flow, the meta authz token and the wif routes.
func newtestclient(t *testing.T, accountID string, wif http.HandlerFunc) *http.Client {
	t.Helper()
	t.Setenv("EG_SESSION_TOKEN", "stub-refresh-token")

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"access_token":"stub-access-token","token_type":"bearer","expires_in":3600}`)
	})
	mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"profiles":[{"token":"the-token","profile":{"id":"profile-1"}}]}`)
	})
	mux.HandleFunc("/authn/current", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"token":"stub-session-token","profile":{"id":"profile-1"},"account":{"id":"account-1"}}`)
	})
	// the meta token carries permissions the compute token does not have.
	mux.HandleFunc("/m/authz/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"token": map[string]any{
			"iss":            accountID,
			"bearer":         "meta-bearer",
			"expires":        time.Now().Add(time.Hour).UnixMilli(),
			"usermanagement": true,
			"billing_read":   true,
			"billing_modify": true,
		}})
	})
	mux.HandleFunc("/m/gcp/wif/", wif)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
	require.NoError(t, err)

	return authn.New(t.Context(), &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}, signer, "", compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))
}

func TestClient(t *testing.T) {
	t.Run("should authorize with the meta token and send it to the wif routes", func(t *testing.T) {
		var authorization string
		c := newtestclient(t, "account-1", func(w http.ResponseWriter, r *http.Request) {
			authorization = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"connection":{"label":"account-1","audience":"aud"}}`)
		})

		conn, err := gcpwif.Create(t.Context(), c, &gcpwif.GCPWIFConnection{Audience: "aud"})
		require.NoError(t, err)
		require.Equal(t, "account-1", conn.GetLabel())
		require.Equal(t, "Bearer meta-bearer", authorization)
	})

	t.Run("should surface the status of a failed request", func(t *testing.T) {
		c := newtestclient(t, "account-1", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})

		_, err := gcpwif.Create(t.Context(), c, &gcpwif.GCPWIFConnection{Audience: "nope"})
		require.ErrorContains(t, err, "400")
	})
}

func TestParsing(t *testing.T) {
	parse := func(t *testing.T, args ...string) error {
		var cli struct {
			Cmd
		}
		parser, err := kong.New(&cli, kong.Vars{"vars_ssh_key_path": "/expected/path"})
		require.NoError(t, err)
		_, err = parser.Parse(args)
		return err
	}

	t.Run("label is optional", func(t *testing.T) {
		require.NoError(t, parse(t, "wif", "create", "--audience", "x"))
		require.NoError(t, parse(t, "wif", "create", "prod", "--audience", "x"))
		require.NoError(t, parse(t, "wif", "edit", "--description", "x"))
		require.NoError(t, parse(t, "wif", "rm"))
		require.NoError(t, parse(t, "wif", "ls"))
	})

	t.Run("create requires an audience", func(t *testing.T) {
		require.Error(t, parse(t, "wif", "create"))
	})

	t.Run("impersonate and clear-impersonate are exclusive", func(t *testing.T) {
		require.Error(t, parse(t, "wif", "edit", "--impersonate", "a@b.iam.gserviceaccount.com", "--clear-impersonate"))
	})
}
