package awswif_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/egdaemon/eg/awswif"
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
	t.Run("should post the connection and return the stored one", func(t *testing.T) {
		var got awswif.AWSWIFCreateRequest
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/m/aws/wif/", r.URL.Path)
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(b, &got))
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"connection":{"label":"account-1","audience":"aud","region":"us-east-2","description":"d"}}`)
		})

		conn, err := awswif.Create(t.Context(), c, &awswif.AWSWIFConnection{Audience: "aud", Region: "us-east-2", Description: "d"})
		require.NoError(t, err)
		require.Equal(t, "aud", got.GetConnection().GetAudience())
		require.Empty(t, got.GetConnection().GetLabel())
		require.Equal(t, "account-1", conn.GetLabel())
		require.Equal(t, "us-east-2", conn.GetRegion())
	})

	t.Run("should return the status of a failed request", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })

		_, err := awswif.Create(t.Context(), c, &awswif.AWSWIFConnection{Audience: "aud"})
		var status *awswif.StatusError
		require.ErrorAs(t, err, &status)
		require.Equal(t, http.StatusForbidden, status.Code)
		require.False(t, awswif.NotFound(err))
	})
}

func TestFind(t *testing.T) {
	t.Run("should get the connection by label", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/m/aws/wif/prod%2Fa", r.URL.EscapedPath())
			fmt.Fprint(w, `{"connection":{"label":"prod/a","audience":"aud"}}`)
		})

		conn, err := awswif.Find(t.Context(), c, "prod/a")
		require.NoError(t, err)
		require.Equal(t, "prod/a", conn.GetLabel())
	})

	t.Run("should report not found", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })

		_, err := awswif.Find(t.Context(), c, "missing")
		require.Error(t, err)
		require.True(t, awswif.NotFound(err))
	})
}

func TestSearch(t *testing.T) {
	t.Run("should send the query and offset and return the page", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/m/aws/wif/", r.URL.Path)
			require.Equal(t, "prod", r.URL.Query().Get("query"))
			require.Equal(t, "3", r.URL.Query().Get("offset"))
			fmt.Fprint(w, `{"next":{"offset":4,"limit":100},"items":[{"label":"a"},{"label":"b"}]}`)
		})

		page, err := awswif.Search(t.Context(), c, "prod", 3)
		require.NoError(t, err)
		require.Len(t, page.GetItems(), 2)
		require.EqualValues(t, 4, page.GetNext().GetOffset())
	})
}

func TestUpdate(t *testing.T) {
	t.Run("should patch only the fields set, including clearing with an empty string", func(t *testing.T) {
		var raw map[string]any
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/m/aws/wif/prod", r.URL.Path)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
			fmt.Fprint(w, `{"connection":{"label":"prod","audience":"aud"}}`)
		})

		empty := ""
		conn, err := awswif.Update(t.Context(), c, "prod", &awswif.AWSWIFUpdateRequest{Region: &empty})
		require.NoError(t, err)
		require.Equal(t, "prod", conn.GetLabel())
		require.Equal(t, map[string]any{"region": ""}, raw)
	})
}

func TestRemove(t *testing.T) {
	t.Run("should delete by label", func(t *testing.T) {
		var method, path string
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			method, path = r.Method, r.URL.Path
		})

		require.NoError(t, awswif.Remove(t.Context(), c, "prod"))
		require.Equal(t, http.MethodDelete, method)
		require.Equal(t, "/m/aws/wif/prod", path)
	})

	t.Run("should report not found", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
		require.True(t, awswif.NotFound(awswif.Remove(t.Context(), c, "missing")))
	})
}

func TestIssuer(t *testing.T) {
	t.Run("should return the issuer and subject", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/m/aws/oidc/", r.URL.Path)
			fmt.Fprint(w, `{"issuer_url":"https://sso.example.com","subject":"account:1"}`)
		})

		rsp, err := awswif.Issuer(t.Context(), c)
		require.NoError(t, err)
		require.Equal(t, "https://sso.example.com", rsp.GetIssuerUrl())
		require.Equal(t, "account:1", rsp.GetSubject())
	})
}
