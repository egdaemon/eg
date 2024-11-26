package compute

import (
	context "context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/encoding/protojson"
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

func (t *Token) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(t)
}

func (t *Token) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, t)
}

func NewAuthzTokenSource(c *http.Client, signer ssh.Signer, endpoint string) TokenSourceFromEndpoint {
	return TokenSourceFromEndpoint{c: c, signer: signer, endpoint: endpoint}
}

type TokenSourceFromEndpoint struct {
	c        *http.Client
	endpoint string
	signer   ssh.Signer
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

	if err = authn.ExchangeAuthed(ctx, chttp, fmt.Sprintf("%s/authn/ssh", eg.EnvAPIHostDefault()), &authed); err != nil {
		return nil, errorsx.Wrap(err, "exchange failed")
	}

	if len(authed.Profiles) != 1 {
		return nil, fmt.Errorf("expected a single profile: %s - %d", authed.Identity.Id, len(authed.Profiles))
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

	if err = json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}

	ts := time.UnixMilli(m.Token.Expires)
	debugx.Println("token expiration", ts, time.Until(ts))
	return &oauth2.Token{TokenType: "BEARER", AccessToken: m.Token.Bearer, Expiry: ts}, nil
}
