package authn

import (
	"context"
	"net/http"

	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

// TokenAuthz builds the token source that exchanges the signer's identity for an authorization
// token from the endpoint, see compute.NewAuthzTokenSource. generic over the token source it
// returns so constructors can return their own type.
type TokenAuthz[T oauth2.TokenSource] func(c *http.Client, signer ssh.Signer, endpoint string, account string) T

type config struct {
	endpoint string
}

// Option configures the client built by New.
type Option func(*config)

// OptionEndpoint the authorization endpoint tokens are fetched from, defaults to EndpointCompute.
func OptionEndpoint(endpoint string) Option {
	return func(c *config) {
		c.endpoint = endpoint
	}
}

// New builds an http client for the eg api that authorizes requests with tokens from ts. c is the
// base client the token requests and the authorized requests are made with, signer is the identity
// to authenticate as, and aid is the account to authorize as, it may be empty when the signer maps
// to a single profile.
func New[T oauth2.TokenSource](ctx context.Context, c *http.Client, signer ssh.Signer, aid string, ts TokenAuthz[T], options ...Option) *http.Client {
	cfg := config{endpoint: EndpointCompute()}
	for _, opt := range options {
		opt(&cfg)
	}

	return oauth2.NewClient(
		context.WithValue(ctx, oauth2.HTTPClient, c),
		ts(c, signer, cfg.endpoint, aid),
	)
}
