package debrepo_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/egdaemon/eg/debrepo"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

// newclient a client whose requests, whatever their host, are served by h.
func newclient(t *testing.T, h http.HandlerFunc) *http.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}
}

func TestCreate(t *testing.T) {
	t.Run("should post the namespace and return the stored one", func(t *testing.T) {
		var got debrepo.CreateRequest
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/deb/n/", r.URL.Path)
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(b, &got))
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"namespace":{"name":"tools","account_id":"acct","public":true}}`)
		})

		n, err := debrepo.Create(t.Context(), c, &debrepo.Namespace{Name: "tools", Public: true, SigningKeys: "keys"})
		require.NoError(t, err)
		require.Equal(t, "tools", got.GetNamespace().GetName())
		require.Equal(t, "keys", got.GetNamespace().GetSigningKeys())
		require.Equal(t, "acct", n.GetAccountId())
	})

	t.Run("should return the reason of a rejected request", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"reason":"invalid namespace name"}`)
		})

		_, err := debrepo.Create(t.Context(), c, &debrepo.Namespace{Name: "Bad"})
		var status *debrepo.StatusError
		require.ErrorAs(t, err, &status)
		require.Equal(t, http.StatusBadRequest, status.Code)
		require.ErrorContains(t, err, "invalid namespace name")
	})
}

func TestFind(t *testing.T) {
	t.Run("should report missing namespaces", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/deb/n/missing", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := debrepo.Find(t.Context(), c, "missing")
		require.True(t, debrepo.NotFound(err))
	})
}

func TestHostname(t *testing.T) {
	n := &debrepo.Namespace{Name: "tools", AccountId: "8e9f2d1a-0000-0000-0000-000000000000"}
	require.Equal(t, "tools.8e9f2d1a-0000-0000-0000-000000000000.apt.egdaemon.com", debrepo.Hostname(n))

	t.Setenv(debrepo.EnvAptHostname, "apt.localhost")
	require.Equal(t, "tools.8e9f2d1a-0000-0000-0000-000000000000.apt.localhost", debrepo.Hostname(n))
}
