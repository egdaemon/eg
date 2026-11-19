package cmdapt

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

	"github.com/egdaemon/eg/cmd/cmdauthz"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/stretchr/testify/require"
)

type request struct {
	method, path, authorization, body string
}

// fakeapi serves the ssh identity exchange, both authorization endpoints (ci tokens are
// ci-bearer, meta tokens meta-bearer) and records every request to the repository.
func fakeapi(t *testing.T) (*cmdopts.Global, *cmdopts.TLSConfig, *cmdidentity.T, func() []request) {
	t.Setenv(userx.EnvCacheDirectory, t.TempDir())
	t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
	t.Setenv("EG_SESSION_TOKEN", "stub-refresh-token")

	var (
		m        sync.Mutex
		requests []request
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
		fmt.Fprintf(w, `{"token":{"iss":"account-1","bearer":"meta-bearer","expires":%d}}`, time.Now().Add(time.Hour).UnixMilli())
	})
	mux.HandleFunc("/c/authz/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"token":{"iss":"account-1","bearer":"ci-bearer","expires":%d}}`, time.Now().Add(time.Hour).UnixMilli())
	})
	mux.HandleFunc("/repo/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.Lock()
		requests = append(requests, request{method: r.Method, path: r.URL.Path, authorization: r.Header.Get("Authorization"), body: string(body)})
		m.Unlock()

		switch r.URL.Path {
		case "/repo/key.asc":
			fmt.Fprint(w, "key")
		case "/repo/publish":
			fmt.Fprint(w, `{"vid":"v1"}`)
		default:
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"filename":"pool/a.deb","sha256":"s","bytes":1,"staged":true}`)
		}
	})
	mux.HandleFunc("/repo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.Lock()
		requests = append(requests, request{method: r.Method, path: r.URL.Path, authorization: r.Header.Get("Authorization"), body: string(body)})
		m.Unlock()
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		fmt.Fprint(w, `{"name":"repo"}`)
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

	recorded := func() []request {
		m.Lock()
		defer m.Unlock()
		return append([]request(nil), requests...)
	}

	return &cmdopts.Global{Context: t.Context(), AccountID: "account-1"}, tlsc, cmdidentity.New(t.Context(), "account-1", "", nil), recorded
}

func newtarget(t *testing.T) target {
	return target{Identity: cmdauthz.Identity{SSHKeyPath: filepath.Join(t.TempDir(), "sshkey")}, AptDomain: "apt.test", Name: "repo"}
}

func TestRun(t *testing.T) {
	t.Run("should create and change repositories with the meta token", func(t *testing.T) {
		gctx, tlsc, ident, recorded := fakeapi(t)
		suite := "testing"
		public := false

		require.NoError(t, Create{target: newtarget(t), settings: settings{Suite: &suite}}.Run(gctx, tlsc, ident))
		keep := 2
		grace := 90 * 24 * time.Hour
		require.NoError(t, Set{target: newtarget(t), settings: settings{Public: &public}, KeepN: &keep, KeepNGrace: &grace}.Run(gctx, tlsc, ident))

		reqs := recorded()
		require.Len(t, reqs, 2)
		require.Equal(t, request{method: http.MethodPost, path: "/repo", authorization: "Bearer meta-bearer", body: `{"suite":"testing"}`}, reqs[0])
		require.Equal(t, http.MethodPatch, reqs[1].method)
		require.Equal(t, "Bearer meta-bearer", reqs[1].authorization)
		require.JSONEq(t, `{"public":false,"keepn":2,"keepn_grace_period":"2160h0m0s"}`, reqs[1].body)
	})

	t.Run("should read, upload and publish with the ci token", func(t *testing.T) {
		gctx, tlsc, ident, recorded := fakeapi(t)

		require.NoError(t, Show{target: newtarget(t)}.Run(gctx, tlsc, ident))
		require.NoError(t, Upload{target: newtarget(t), Debs: debs(t, "a.deb"), Publish: true, Retries: 1}.Run(gctx, tlsc, ident))
		require.NoError(t, Publish{target: newtarget(t)}.Run(gctx, tlsc, ident))

		reqs := recorded()
		require.Len(t, reqs, 4)
		for i, expected := range []struct{ method, path string }{
			{http.MethodGet, "/repo"},
			{http.MethodPut, "/repo/upload/a.deb"},
			{http.MethodPost, "/repo/publish"},
			{http.MethodPost, "/repo/publish"},
		} {
			require.Equal(t, expected.method, reqs[i].method)
			require.Equal(t, expected.path, reqs[i].path)
			require.Equal(t, "Bearer ci-bearer", reqs[i].authorization)
		}
	})

	t.Run("should fetch the key without authorization", func(t *testing.T) {
		gctx, tlsc, ident, recorded := fakeapi(t)
		output := filepath.Join(t.TempDir(), "repo.asc")

		require.NoError(t, Key{target: newtarget(t), Output: output}.Run(gctx, tlsc, ident))

		reqs := recorded()
		require.Len(t, reqs, 1)
		require.Equal(t, "/repo/key.asc", reqs[0].path)
		require.Empty(t, reqs[0].authorization)
		require.FileExists(t, output)
	})

	t.Run("should reject invalid repository names before any request", func(t *testing.T) {
		gctx, tlsc, ident, recorded := fakeapi(t)
		tgt := newtarget(t)
		tgt.Name = "Bad_Name"

		require.Error(t, Show{target: tgt}.Run(gctx, tlsc, ident))
		require.Empty(t, recorded())
	})
}
