package authn_test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/stretchr/testify/require"
)

func TestLoginOptionsToken(t *testing.T) {
	request := func(t *testing.T, options ...authn.LoginOption) url.Values {
		var query url.Values

		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/authn/s/", r.URL.Path)
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"t"}`)), Header: http.Header{}}, nil
		})}

		token, err := authn.LoginOptionsToken(t.Context(), c, "account-1", options...)
		require.NoError(t, err)
		require.JSONEq(t, `{"token":"t"}`, string(token))
		return query
	}

	t.Run("requests the options of the account", func(t *testing.T) {
		query := request(t)
		require.Equal(t, "account-1", query.Get("account_id"))
		require.False(t, query.Has("mid"))
	})

	t.Run("asserts the machine id", func(t *testing.T) {
		query := request(t, authn.LoginOptionMachineID("machine-1"))
		require.Equal(t, "account-1", query.Get("account_id"))
		require.Equal(t, "machine-1", query.Get("mid"))
	})

	t.Run("does not send a blank machine id", func(t *testing.T) {
		require.False(t, request(t, authn.LoginOptionMachineID("")).Has("mid"))
	})
}
