package compute

import (
	"bytes"
	context "context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/egdaemon/eg/jsonx"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

func (t *Token) Valid() error {
	return jwt.RegisteredClaims{
		ID:        t.Id,
		Issuer:    t.AccountId,
		Subject:   t.ProfileId,
		IssuedAt:  jwt.NewNumericDate(time.Unix(t.Issued, 0)),
		ExpiresAt: jwt.NewNumericDate(time.Unix(t.Expires, 0)),
		NotBefore: jwt.NewNumericDate(time.Unix(t.NotBefore, 0)),
	}.Valid()
}

// UnmarshalJSON accepts the numbers of the jwt claims (iat, exp, nbf, compute_remaining) as well as
// the strings of the api responses; the jwt library decodes claims with encoding/json, which
// rejects numbers for the fields tagged as strings.
func (t *Token) UnmarshalJSON(b []byte) error {
	return jsonx.Unmarshal(b, t)
}

func NewAuthzTokenSource(c *http.Client, signer ssh.Signer, endpoint string, account string, profile string) TokenSourceFromEndpoint {
	return TokenSourceFromEndpoint{c: c, signer: signer, endpoint: endpoint, account: account, profile: profile}
}

// errAmbiguous is identity.ErrAmbiguous. identity registers the cgo duckdb driver, which cannot be built for the
// wasm workloads that import this package (via gitx), so the sentinel is repeated here. errors.Is matches them
// as they are equal values of the same type.
const errAmbiguous = errorsx.String("multiple profiles are associated with this ssh key")

type TokenSourceFromEndpoint struct {
	c         *http.Client
	endpoint  string
	signer    ssh.Signer
	account   string
	profile   string
	machineid string
}

// WithMachineID asserts the machine id of the runner the tokens are for, so the tokens identify the exact
// runner when the machines of an account share a key.
func (t TokenSourceFromEndpoint) WithMachineID(mid string) TokenSourceFromEndpoint {
	t.machineid = mid
	return t
}

func (t TokenSourceFromEndpoint) Token() (_ *oauth2.Token, err error) {
	type msg struct {
		Token Token `json:"token"`
	}
	var (
		authed  authn.Authed
		m       msg
		encoded string
	)

	debugx.Println("token refresh initiated")
	defer debugx.Println("token refresh completed")
	defer func() {
		errorsx.Log(err)
	}()

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	if encoded, err = authn.AutoTokenState(t.signer); err != nil {
		return nil, errorsx.Wrap(err, "unable to generated token state")
	}

	refreshtoken, err := authn.AutoRefreshTokenState(context.WithValue(ctx, oauth2.HTTPClient, t.c), t.signer, encoded)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to generate refresh token")
	}

	cfg := authn.OAuth2SSHConfig(t.signer, "", authn.EndpointSSHAuth())
	chttp := cfg.Client(context.WithValue(ctx, oauth2.HTTPClient, t.c), refreshtoken)

	var options io.Reader
	if t.account != "" || t.profile != "" {
		token, err := authn.LoginOptionsToken(ctx, chttp, authn.LoginOptionAccountID(t.account), authn.LoginOptionProfileID(t.profile), authn.LoginOptionMachineID(t.machineid))
		if err != nil {
			return nil, errorsx.Wrap(err, "unable to fetch login options")
		}
		options = bytes.NewReader(token)
	}

	if err = authn.ExchangeAuthed(ctx, chttp, fmt.Sprintf("%s/authn/ssh", eg.EnvAPIHostDefault()), options, &authed); err != nil {
		return nil, errorsx.Wrap(err, "exchange failed")
	}

	switch len(authed.Profiles) {
	case 1:
	case 0:
		return nil, fmt.Errorf("identity %s is not associated with any profiles", authed.Identity.Id)
	default:
		return nil, errorsx.Wrapf(errAmbiguous, "identity %s has %d profiles, specify one with --account-id or EG_ACCOUNT, or pick a profile with --profile-id or EG_PROFILE", authed.Identity.Id, len(authed.Profiles))
	}

	session, err := authn.Session(ctx, t.c, authed.Profiles[0].Token)
	if err != nil {
		return nil, errorsx.Wrap(err, "session creation failed")
	}

	chttp = cfg.Client(context.WithValue(ctx, oauth2.HTTPClient, t.c), &oauth2.Token{
		TokenType:   "BEARER",
		AccessToken: session.Token,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpx.AsError(chttp.Do(req))
	if err != nil {
		return nil, err
	}
	defer httpx.AutoClose(resp)

	if err = jsonx.UnmarshalRead(resp.Body, &m); err != nil {
		return nil, err
	}

	ts := time.UnixMilli(m.Token.Expires).Add(-10 * time.Second)
	debugx.Println("token expiration", ts, time.Until(ts))
	return &oauth2.Token{TokenType: "BEARER", AccessToken: m.Token.Bearer, Expiry: ts}, nil
}
