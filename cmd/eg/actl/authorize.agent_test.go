package actl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

// authzServerOptions backs the endpoints TokenSourceFromEndpoint.Token() and
// registration.RegistrationClient.Grant() drive. Only the handlers a given
// test cares about need to be supplied -- everything else in the chain
// (oauth2 refresh, the session lookup, the compute token endpoint) is
// defaulted to a successful response so tests can focus on the branch under
// test.
type authzServerOptions struct {
	// Authed builds the response for POST /authn/ssh. Required.
	Authed func() authn.Authed
	// LoginOptions, when set, backs GET /authn/s/ and marks it as expected to
	// be called (an unexpected call otherwise fails the test).
	LoginOptions func(w http.ResponseWriter, r *http.Request)
	// Grant, when set, backs POST /eg/registration/authz. Required whenever a
	// single profile is expected to make it through to the grant call.
	Grant func(w http.ResponseWriter, r *http.Request)
}

// newauthzclient starts a fake server backing the full authorize network
// chain and returns an *http.Client rewritten to talk to it, mirroring
// newtestclient in runners/scheduler.autodownload_test.go. It also seeds
// EG_SESSION_TOKEN so authn.AutoRefreshToken short-circuits the interactive
// browser auth-code flow.
func newauthzclient(t *testing.T, opts authzServerOptions) *http.Client {
	t.Helper()
	t.Setenv("EG_SESSION_TOKEN", "stub-refresh-token")

	mux := http.NewServeMux()

	mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"stub-access-token","token_type":"bearer","expires_in":3600}`)
	})

	mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) {
		if opts.Authed == nil {
			t.Fatal("unexpected call to /authn/ssh")
			return
		}
		encoded, err := json.Marshal(opts.Authed())
		if err != nil {
			t.Fatal(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(encoded)
	})

	mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) {
		if opts.LoginOptions == nil {
			t.Fatal("unexpected call to /authn/s/")
			return
		}
		opts.LoginOptions(w, r)
	})

	mux.HandleFunc("/authn/current", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"token":"stub-session-token","profile":{"id":"profile-1"},"account":{"id":"account-1"}}`)
	})

	mux.HandleFunc("/c/authz/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"token":{"bearer":"stub-bearer"}}`)
	})

	mux.HandleFunc("/eg/registration/authz", func(w http.ResponseWriter, r *http.Request) {
		if opts.Grant == nil {
			t.Fatal("unexpected call to /eg/registration/authz")
			return
		}
		opts.Grant(w, r)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dst := errorsx.Must(url.Parse(srv.URL))
	return &http.Client{Transport: httpx.RewriteHostTransport(dst, nil)}
}
