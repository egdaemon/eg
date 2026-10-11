package gitx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egdaemon/eg/compute"
	"github.com/stretchr/testify/require"
)

type roundtripper func(*http.Request) (*http.Response, error)

func (t roundtripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return t(r)
}

func TestExchangeCredentials(t *testing.T) {
	t.Run("exchanges the token for http credentials", func(t *testing.T) {
		var exchanged []string
		c := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.True(t, strings.HasSuffix(r.URL.Path, "/r/vcsaccess/"), r.URL.Path)
			exchanged = append(exchanged, r.Header.Get("Authorization"))
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		auth, err := ExchangeCredentials(t.Context(), c, "https://github.com/example/repo.git", "token")
		require.NoError(t, err)
		require.NotNil(t, auth)
		require.Equal(t, []string{"BEARER token"}, exchanged)
	})

	t.Run("blank token errors without an exchange", func(t *testing.T) {
		c := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			t.Errorf("unexpected request: %s", r.URL.Path)
			return httptest.NewRecorder().Result(), nil
		})}

		_, err := ExchangeCredentials(t.Context(), c, "https://github.com/example/repo.git", "")
		require.Error(t, err)
	})

	t.Run("non-http uris receive no credentials", func(t *testing.T) {
		c := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		auth, err := ExchangeCredentials(t.Context(), c, "git@github.com:example/repo.git", "token")
		require.NoError(t, err)
		require.Nil(t, auth)
	})

	t.Run("failed exchanges error", func(t *testing.T) {
		c := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			w.WriteHeader(http.StatusUnauthorized)
			return w.Result(), nil
		})}

		_, err := ExchangeCredentials(t.Context(), c, "https://github.com/example/repo.git", "token")
		require.Error(t, err)
	})
}
