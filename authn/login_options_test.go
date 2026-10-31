package authn_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/stretchr/testify/require"
)

func TestLoginOptionsToken(t *testing.T) {
	t.Run("requests the options of the account", func(t *testing.T) {
		var seen *http.Request
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			seen = r
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"t"}`)), Header: http.Header{}}, nil
		})}

		token, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"))
		require.NoError(t, err)
		require.JSONEq(t, `{"token":"t"}`, string(token))

		sso, err := url.Parse(eg.EnvSSOHostDefault())
		require.NoError(t, err)
		require.Equal(t, http.MethodGet, seen.Method)
		require.Equal(t, sso.Scheme, seen.URL.Scheme)
		require.Equal(t, sso.Host, seen.URL.Host)
		require.Equal(t, "/authn/s/", seen.URL.Path)
		require.Equal(t, url.Values{"account_id": {"account-1"}}, seen.URL.Query())
	})

	t.Run("does not send a blank account id", func(t *testing.T) {
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID(""))
		require.NoError(t, err)
		require.False(t, query.Has("account_id"))
	})

	t.Run("escapes the account id", func(t *testing.T) {
		var raw string
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			raw = r.URL.RawQuery
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("a&b=c d/é"))
		require.NoError(t, err)
		require.Equal(t, "a&b=c d/é", query.Get("account_id"))
		require.Len(t, query, 1)
		require.NotContains(t, raw, " ")
		require.NotContains(t, raw, "&b=")
	})

	t.Run("returns the body verbatim", func(t *testing.T) {
		body := " {\"token\" : \"t\",\n \"extra\":[1,2]} \n"
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
		})}

		token, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"))
		require.NoError(t, err)
		require.Equal(t, body, string(token))
	})

	t.Run("returns an empty body", func(t *testing.T) {
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		})}

		token, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"))
		require.NoError(t, err)
		require.Empty(t, token)
	})

	t.Run("fails on non success status", func(t *testing.T) {
		for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError} {
			c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
			})}

			token, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"))
			require.Error(t, err, "status %d", status)
			require.Nil(t, token)
		}
	})

	t.Run("propagates transport errors", func(t *testing.T) {
		expected := errors.New("transport failure")
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			return nil, expected
		})}

		token, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"))
		require.ErrorIs(t, err, expected)
		require.Nil(t, token)
	})

	t.Run("honors context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			return nil, r.Context().Err()
		})}

		token, err := authn.LoginOptionsToken(ctx, c, authn.LoginOptionAccountID("account-1"))
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, token)
	})

	t.Run("propagates the context to the request", func(t *testing.T) {
		type ctxkey struct{}
		var seen any
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			seen = r.Context().Value(ctxkey{})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(context.WithValue(t.Context(), ctxkey{}, "marker"), c, authn.LoginOptionAccountID("account-1"))
		require.NoError(t, err)
		require.Equal(t, "marker", seen)
	})
}

func TestLoginOption(t *testing.T) {
	t.Run("without options nothing is sent", func(t *testing.T) {
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c)
		require.NoError(t, err)
		require.Empty(t, query)
	})

	t.Run("no options only sends the account id", func(t *testing.T) {
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"))
		require.NoError(t, err)
		require.Equal(t, url.Values{"account_id": {"account-1"}}, query)
	})

	t.Run("machine id is sent alongside the account id", func(t *testing.T) {
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"), authn.LoginOptionMachineID("machine-1"))
		require.NoError(t, err)
		require.Equal(t, url.Values{"account_id": {"account-1"}, "mid": {"machine-1"}}, query)
	})

	t.Run("blank machine id is not sent", func(t *testing.T) {
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"), authn.LoginOptionMachineID(""))
		require.NoError(t, err)
		require.False(t, query.Has("mid"))
	})

	t.Run("blank machine id does not clear an earlier one", func(t *testing.T) {
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"), authn.LoginOptionMachineID("machine-1"), authn.LoginOptionMachineID(""))
		require.NoError(t, err)
		require.Equal(t, "machine-1", query.Get("mid"))
	})

	t.Run("later machine id overrides an earlier one", func(t *testing.T) {
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"), authn.LoginOptionMachineID("machine-1"), authn.LoginOptionMachineID("machine-2"))
		require.NoError(t, err)
		require.Equal(t, []string{"machine-2"}, query["mid"])
	})

	t.Run("options are applied in order", func(t *testing.T) {
		var order []string
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"),
			func(u *url.URL) {
				q := u.Query()
				order = append(order, "first")
				q.Set("k", "first")
				u.RawQuery = q.Encode()
			},
			func(u *url.URL) {
				q := u.Query()
				order = append(order, "second")
				q.Set("k", "second")
				u.RawQuery = q.Encode()
			},
			func(u *url.URL) {
				q := u.Query()
				order = append(order, "third")
				q.Add("k", "third")
				u.RawQuery = q.Encode()
			},
		)
		require.NoError(t, err)
		require.Equal(t, []string{"first", "second", "third"}, order)
		require.Equal(t, []string{"second", "third"}, query["k"])
	})

	t.Run("custom options can add arbitrary keys", func(t *testing.T) {
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"), func(u *url.URL) { q := u.Query(); q.Set("custom", "v a/l"); u.RawQuery = q.Encode() })
		require.NoError(t, err)
		require.Equal(t, url.Values{"account_id": {"account-1"}, "custom": {"v a/l"}}, query)
	})

	t.Run("custom options can override the account id", func(t *testing.T) {
		var query url.Values
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			query = r.URL.Query()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, authn.LoginOptionAccountID("account-1"), func(u *url.URL) { q := u.Query(); q.Set("account_id", "account-2"); u.RawQuery = q.Encode() })
		require.NoError(t, err)
		require.Equal(t, []string{"account-2"}, query["account_id"])
	})
}

func TestLoginOptionSubdomain(t *testing.T) {
	request := func(t *testing.T, options ...authn.LoginOption) *url.URL {
		var seen *url.URL
		c := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			seen = r.URL
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		})}

		_, err := authn.LoginOptionsToken(t.Context(), c, append([]authn.LoginOption{authn.LoginOptionAccountID("account-1")}, options...)...)
		require.NoError(t, err)
		return seen
	}

	sso, err := url.Parse(eg.EnvSSOHostDefault())
	require.NoError(t, err)

	t.Run("prefixes the sso host", func(t *testing.T) {
		seen := request(t, authn.LoginOptionSubdomain("acme"))
		require.Equal(t, "acme.sso.egdaemon.com", seen.Host)
		require.Equal(t, sso.Scheme, seen.Scheme)
		require.Equal(t, "/authn/s/", seen.Path)
		require.Equal(t, "account-1", seen.Query().Get("account_id"))
	})

	t.Run("blank is ignored", func(t *testing.T) {
		require.Equal(t, sso.Host, request(t, authn.LoginOptionSubdomain("")).Host)
	})

	t.Run("composes with other options", func(t *testing.T) {
		seen := request(t, authn.LoginOptionMachineID("machine-1"), authn.LoginOptionSubdomain("acme"))
		require.Equal(t, "acme.sso.egdaemon.com", seen.Host)
		require.Equal(t, "machine-1", seen.Query().Get("mid"))
		require.Equal(t, "account-1", seen.Query().Get("account_id"))
	})

	t.Run("idempotent", func(t *testing.T) {
		once := request(t, authn.LoginOptionSubdomain("acme"))
		twice := request(t, authn.LoginOptionSubdomain("acme"), authn.LoginOptionSubdomain("acme"))
		require.Equal(t, once.Host, twice.Host)
	})

	t.Run("last subdomain wins", func(t *testing.T) {
		seen := request(t, authn.LoginOptionSubdomain("acme"), authn.LoginOptionSubdomain("other"))
		require.Equal(t, "other.sso.egdaemon.com", seen.Host)
	})

	t.Run("order independent", func(t *testing.T) {
		a := request(t, authn.LoginOptionSubdomain("acme"), authn.LoginOptionMachineID("machine-1"))
		b := request(t, authn.LoginOptionMachineID("machine-1"), authn.LoginOptionSubdomain("acme"))
		require.Equal(t, a.String(), b.String())
	})
}
