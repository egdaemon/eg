package authn_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type roundtrip func(*http.Request) (*http.Response, error)

func (t roundtrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return t(r)
}

func newsigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return signer
}

type tokenauthz struct {
	c        *http.Client
	signer   ssh.Signer
	endpoint string
	account  string
}

func TestNew(t *testing.T) {
	t.Run("should build the token source from the arguments and default endpoint", func(t *testing.T) {
		var (
			got    tokenauthz
			signer = newsigner(t)
			base   = &http.Client{}
		)

		authn.New(t.Context(), base, signer, "account-1", func(c *http.Client, s ssh.Signer, endpoint string, account string) oauth2.TokenSource {
			got = tokenauthz{c: c, signer: s, endpoint: endpoint, account: account}
			return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token"})
		})

		require.Same(t, base, got.c)
		require.Equal(t, signer, got.signer)
		require.Equal(t, authn.EndpointCompute(), got.endpoint)
		require.Equal(t, "account-1", got.account)
	})

	t.Run("should use the endpoint option", func(t *testing.T) {
		var got tokenauthz

		authn.New(t.Context(), &http.Client{}, newsigner(t), "", func(c *http.Client, s ssh.Signer, endpoint string, account string) oauth2.TokenSource {
			got = tokenauthz{endpoint: endpoint, account: account}
			return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token"})
		}, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

		require.Equal(t, authn.EndpointMetaAuthz(), got.endpoint)
		require.Empty(t, got.account)
	})

	t.Run("should authorize requests with the token using the base client", func(t *testing.T) {
		var seen *http.Request

		base := &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			seen = r
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{}}, nil
		})}

		c := authn.New(t.Context(), base, newsigner(t), "account-1", func(*http.Client, ssh.Signer, string, string) oauth2.TokenSource {
			return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token", TokenType: "Bearer"})
		})

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/", nil)
		require.NoError(t, err)

		resp, err := c.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.NotNil(t, seen)
		require.Equal(t, "Bearer token", seen.Header.Get("Authorization"))
	})

	t.Run("should surface token source errors on the first request", func(t *testing.T) {
		c := authn.New(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			t.Fatal("request must not be sent without a token")
			return nil, nil
		})}, newsigner(t), "account-1", func(*http.Client, ssh.Signer, string, string) oauth2.TokenSource {
			return tokenerror{}
		})

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/", nil)
		require.NoError(t, err)

		_, err = c.Do(req)
		require.ErrorContains(t, err, "deadline exceeded")
	})
}

type tokenerror struct{}

func (tokenerror) Token() (*oauth2.Token, error) {
	return nil, context.DeadlineExceeded
}
