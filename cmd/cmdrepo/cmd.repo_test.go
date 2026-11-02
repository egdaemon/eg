package cmdrepo

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/stretchr/testify/require"
)

const (
	oneprofile  = `{"profiles":[{"token":"t1","profile":{"id":"profile-1","display":"dave","account_id":"acct-001"},"account":{"id":"acct-001"}}]}`
	twoprofiles = `{"profiles":[{"token":"t1","profile":{"id":"profile-1","display":"dave","account_id":"acct-001"},"account":{"id":"acct-001"}},{"token":"t2","profile":{"id":"profile-2","display":"erin","account_id":"acct-002"},"account":{"id":"acct-002"}}]}`
	vcsuri      = "git@github.com:u/r.git"
)

func TestWhoami(t *testing.T) {
	t.Run("single profile is the account", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, oneprofile) })

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

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		// the chooser must never be reached, the tests are not interactive.
		ident := cmdidentity.New(t.Context(), "", "", func([]*authn.Authn) (*authn.Authn, error) {
			t.Fatal("unexpected chooser invocation")
			return nil, nil
		})
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		cmd := Whoami{SSHKeyPath: sshkey}
		cmd.VcsURI = vcsuri

		require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context()}, tlsc, ident))
		require.Contains(t, logged.String(), "repository "+vcsuri)
		require.Contains(t, logged.String(), "account acct-001 - single")
		require.Contains(t, logged.String(), "current acct-001 - dave")
	})

	t.Run("explicit account is reported with its profile", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, twoprofiles) })

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

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		// the chooser must never be reached, the tests are not interactive.
		ident := cmdidentity.New(t.Context(), "", "", func([]*authn.Authn) (*authn.Authn, error) {
			t.Fatal("unexpected chooser invocation")
			return nil, nil
		})
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		cmd := Whoami{SSHKeyPath: sshkey}
		cmd.VcsURI = vcsuri

		require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "acct-002"}, tlsc, ident))
		require.Contains(t, logged.String(), "account acct-002 - flag")
		require.Contains(t, logged.String(), "current acct-002 - erin")
	})

	t.Run("ambiguous accounts are listed as candidates", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, twoprofiles) })

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

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		// the chooser must never be reached, the tests are not interactive.
		ident := cmdidentity.New(t.Context(), "", "", func([]*authn.Authn) (*authn.Authn, error) {
			t.Fatal("unexpected chooser invocation")
			return nil, nil
		})
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		cmd := Whoami{SSHKeyPath: sshkey}
		cmd.VcsURI = vcsuri

		require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context()}, tlsc, ident))
		require.Contains(t, logged.String(), "account ambiguous, candidates:")
		require.Contains(t, logged.String(), "profile acct-001 - dave")
		require.Contains(t, logged.String(), "profile acct-002 - erin")
	})
}

func TestDefaultAccount(t *testing.T) {
	t.Run("requires a repository", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, oneprofile) })

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

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		// the chooser must never be reached, the tests are not interactive.
		ident := cmdidentity.New(t.Context(), "", "", func([]*authn.Authn) (*authn.Authn, error) {
			t.Fatal("unexpected chooser invocation")
			return nil, nil
		})
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		cmd := DefaultAccount{SSHKeyPath: sshkey}

		err := cmd.Run(&cmdopts.Global{Context: t.Context()}, tlsc, ident)
		require.ErrorContains(t, err, "unable to detect the repository")
	})

	t.Run("an associated account id is remembered for the repository", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, twoprofiles) })

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

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		// the chooser must never be reached, the tests are not interactive.
		ident := cmdidentity.New(t.Context(), "", "", func([]*authn.Authn) (*authn.Authn, error) {
			t.Fatal("unexpected chooser invocation")
			return nil, nil
		})
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		cmd := DefaultAccount{SSHKeyPath: sshkey, ID: "acct-002"}
		cmd.VcsURI = vcsuri

		require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context()}, tlsc, ident))
		require.Contains(t, logged.String(), "account acct-002 will be used for "+vcsuri)

		signer, err := sshx.AutoCached(sshx.NewKeyGen(), sshkey)
		require.NoError(t, err)
		db, err := ident.Store()
		require.NoError(t, err)
		defer db.Close()

		remembered, err := identity.Remembered(t.Context(), db, signer, vcsuri)
		require.NoError(t, err)
		require.Equal(t, "acct-002", remembered)
	})

	t.Run("an account the key is not associated with is rejected", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, twoprofiles) })

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

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		// the chooser must never be reached, the tests are not interactive.
		ident := cmdidentity.New(t.Context(), "", "", func([]*authn.Authn) (*authn.Authn, error) {
			t.Fatal("unexpected chooser invocation")
			return nil, nil
		})
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		cmd := DefaultAccount{SSHKeyPath: sshkey, ID: "acct-999"}
		cmd.VcsURI = vcsuri

		err := cmd.Run(&cmdopts.Global{Context: t.Context()}, tlsc, ident)
		require.ErrorContains(t, err, "is not associated with account acct-999")
	})

	t.Run("detection ignores the remembered account", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		t.Setenv("EG_SESSION_TOKEN", "refresh")

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, oneprofile) })

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

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		// the chooser must never be reached, the tests are not interactive.
		ident := cmdidentity.New(t.Context(), "", "", func([]*authn.Authn) (*authn.Authn, error) {
			t.Fatal("unexpected chooser invocation")
			return nil, nil
		})
		sshkey := filepath.Join(t.TempDir(), "sshkey")

		signer, err := sshx.AutoCached(sshx.NewKeyGen(), sshkey)
		require.NoError(t, err)
		db, err := ident.Store()
		require.NoError(t, err)
		require.NoError(t, identity.Remember(t.Context(), db, signer, vcsuri, "remembered"))
		require.NoError(t, db.Close())

		cmd := DefaultAccount{SSHKeyPath: sshkey}
		cmd.VcsURI = vcsuri

		require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context()}, tlsc, ident))
		require.Contains(t, logged.String(), "account acct-001 - single - will be used for "+vcsuri)
	})
}
