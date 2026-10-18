package cmdgcp

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/kong"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// fake api serving the ssh login flow, the meta authz token and the wif routes.
func newtestclient(t *testing.T, accountID string, wif http.HandlerFunc) *client {
	t.Helper()
	t.Setenv("EG_SESSION_TOKEN", "stub-refresh-token")

	bearer, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{Issuer: accountID}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

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
	mux.HandleFunc("/m/authz/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"token": map[string]any{"bearer": bearer, "expires": time.Now().Add(time.Hour).UnixMilli(), "usermanagement": true, "billing_read": true, "billing_modify": true}})
	})
	mux.HandleFunc("/m/gcp/wif/", wif)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
	require.NoError(t, err)

	c, err := newClientFrom(t.Context(), &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}, signer, "")
	require.NoError(t, err)
	return c
}

func TestClient(t *testing.T) {
	t.Run("label defaults to the account id of the profile", func(t *testing.T) {
		c := newtestclient(t, "account-1", func(w http.ResponseWriter, r *http.Request) {})
		require.Equal(t, "account-1", c.label(""))
		require.Equal(t, "prod", c.label("prod"))
	})

	t.Run("create sends the defaulted label and surfaces the status", func(t *testing.T) {
		var got GCPWIFCreateRequest
		c := newtestclient(t, "account-1", func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Contains(t, r.Header.Get("Authorization"), "Bearer ey")
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, protojson.Unmarshal(b, &got))
			w.WriteHeader(http.StatusBadRequest)
		})

		err := c.do(t.Context(), http.MethodPost, "", nil, &GCPWIFCreateRequest{Connection: &GCPWIFConnection{Label: c.label(""), Audience: "nope"}}, nil)
		require.ErrorContains(t, err, "400")
		require.Equal(t, "account-1", got.GetConnection().GetLabel())
	})

	t.Run("edit and remove address the label in the path", func(t *testing.T) {
		var method, path string
		c := newtestclient(t, "account-1", func(w http.ResponseWriter, r *http.Request) {
			method, path = r.Method, r.URL.Path
			fmt.Fprint(w, `{}`)
		})

		require.NoError(t, c.do(t.Context(), http.MethodPatch, c.label(""), nil, &GCPWIFUpdateRequest{}, nil))
		require.Equal(t, http.MethodPatch, method)
		require.Equal(t, "/m/gcp/wif/account-1", path)

		require.NoError(t, c.do(t.Context(), http.MethodDelete, "prod", nil, nil, nil))
		require.Equal(t, http.MethodDelete, method)
		require.Equal(t, "/m/gcp/wif/prod", path)
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
