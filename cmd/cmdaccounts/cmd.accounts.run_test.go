package cmdaccounts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

const (
	oneprofile  = `{"profiles":[{"token":"token-1","profile":{"id":"profile-1","display":"dave","account_id":"acct-001"}}]}`
	twoprofiles = `{"profiles":[{"token":"token-1","profile":{"id":"profile-1","display":"dave","account_id":"acct-001"}},{"token":"token-2","profile":{"id":"profile-2","display":"erin","account_id":"acct-002"}}]}`
	noprofiles  = `{"profiles":[]}`
)

func TestLoginRunChoosesThroughIdentity(t *testing.T) {
	t.Run("a single profile logs in without asking", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: "stub-options-token"})
		require.NoError(t, err)

		var (
			m        sync.Mutex
			requests []string
		)

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(options) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, oneprofile) })
		mux.HandleFunc("/authn/", func(w http.ResponseWriter, r *http.Request) {
			m.Lock()
			defer m.Unlock()
			requests = append(requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"profile":{"id":"profile-1","display":"dave"},"account":{"id":"acct-001","display":"acme"}}`)
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
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		ident := cmdidentity.New(t.Context(), "", func([]*authn.Authn) (*authn.Authn, error) {
			t.Fatal("unexpected chooser invocation")
			return nil, nil
		})

		require.NoError(t, Login{SSHKeyPath: sshkey}.Run(&cmdopts.Global{Context: t.Context()}, tlsc, ident))

		m.Lock()
		defer m.Unlock()
		require.Equal(t, []string{"GET /authn/current BEARER token-1"}, requests)
	})

	t.Run("multiple profiles log in as the one the identity chooses", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: "stub-options-token"})
		require.NoError(t, err)

		var (
			m        sync.Mutex
			requests []string
		)

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(options) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, twoprofiles) })
		mux.HandleFunc("/authn/", func(w http.ResponseWriter, r *http.Request) {
			m.Lock()
			defer m.Unlock()
			requests = append(requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"profile":{"id":"profile-1","display":"dave"},"account":{"id":"acct-001","display":"acme"}}`)
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
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		var offered []*authn.Authn
		ident := cmdidentity.New(t.Context(), "", func(profiles []*authn.Authn) (*authn.Authn, error) {
			offered = profiles
			return profiles[1], nil
		})

		require.NoError(t, Login{SSHKeyPath: sshkey}.Run(&cmdopts.Global{Context: t.Context()}, tlsc, ident))
		require.Len(t, offered, 2)

		m.Lock()
		defer m.Unlock()
		require.Equal(t, []string{"GET /authn/current BEARER token-2"}, requests)
	})

	t.Run("a chooser failure is returned and no session is established", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: "stub-options-token"})
		require.NoError(t, err)

		var (
			m        sync.Mutex
			requests []string
		)

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(options) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, twoprofiles) })
		mux.HandleFunc("/authn/", func(w http.ResponseWriter, r *http.Request) {
			m.Lock()
			defer m.Unlock()
			requests = append(requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"profile":{"id":"profile-1","display":"dave"},"account":{"id":"acct-001","display":"acme"}}`)
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
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		ident := cmdidentity.New(t.Context(), "", func([]*authn.Authn) (*authn.Authn, error) {
			return nil, identity.ErrAmbiguous
		})

		err = Login{SSHKeyPath: sshkey}.Run(&cmdopts.Global{Context: t.Context()}, tlsc, ident)
		require.ErrorIs(t, err, identity.ErrAmbiguous)

		m.Lock()
		defer m.Unlock()
		require.Empty(t, requests)
	})
}

func TestOTPRun(t *testing.T) {
	t.Run("a single profile prints the browser login", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: "stub-options-token"})
		require.NoError(t, err)

		var (
			m        sync.Mutex
			requests []string
		)

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(options) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, oneprofile) })
		mux.HandleFunc("/authn/", func(w http.ResponseWriter, r *http.Request) {
			m.Lock()
			defer m.Unlock()
			requests = append(requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"profile":{"id":"profile-1","display":"dave"},"account":{"id":"acct-001","display":"acme"}}`)
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
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		require.NoError(t, OTP{SSHKeyPath: sshkey}.Run(&cmdopts.Global{Context: t.Context()}, tlsc))

		m.Lock()
		defer m.Unlock()
		require.Empty(t, requests)
	})

	t.Run("multiple profiles are not supported yet", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: "stub-options-token"})
		require.NoError(t, err)

		var (
			m        sync.Mutex
			requests []string
		)

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(options) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, twoprofiles) })
		mux.HandleFunc("/authn/", func(w http.ResponseWriter, r *http.Request) {
			m.Lock()
			defer m.Unlock()
			requests = append(requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"profile":{"id":"profile-1","display":"dave"},"account":{"id":"acct-001","display":"acme"}}`)
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
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		err = OTP{SSHKeyPath: sshkey}.Run(&cmdopts.Global{Context: t.Context()}, tlsc)
		require.ErrorContains(t, err, "multiple account support")
	})

	t.Run("no profiles is a notification", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: "stub-options-token"})
		require.NoError(t, err)

		var (
			m        sync.Mutex
			requests []string
		)

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(options) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, noprofiles) })
		mux.HandleFunc("/authn/", func(w http.ResponseWriter, r *http.Request) {
			m.Lock()
			defer m.Unlock()
			requests = append(requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"profile":{"id":"profile-1","display":"dave"},"account":{"id":"acct-001","display":"acme"}}`)
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
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		err = OTP{SSHKeyPath: sshkey}.Run(&cmdopts.Global{Context: t.Context()}, tlsc)
		require.ErrorContains(t, err, "is not associated with any profiles")
	})
}

func TestSignupRunSignsUpWithoutProfiles(t *testing.T) {
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": ""}).SignedString([]byte("secret"))
	require.NoError(t, err)

	t.Run("a new key signs up for a new account", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		var (
			m        sync.Mutex
			requests []string
		)

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(options) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, noprofiles) })
		mux.HandleFunc("/authn/", func(w http.ResponseWriter, r *http.Request) {
			m.Lock()
			defer m.Unlock()
			requests = append(requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"profile":{"id":"profile-1","display":"dave"},"account":{"id":"acct-001","display":"acme"}}`)
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
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		err = Signup{SSHKeyPath: sshkey, Name: "dave", Email: "dave@example.com"}.Run(&cmdopts.Global{Context: t.Context()}, tlsc)
		require.NoError(t, err)

		m.Lock()
		defer m.Unlock()
		require.Len(t, requests, 1)
		require.Regexp(t, "^POST /authn/signup BEARER ", requests[0])
	})
}
