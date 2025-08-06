package authn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

func EndpointSSHAuth() oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:   fmt.Sprintf("%s/oauth2/ssh/auth", eg.EnvAPIHostDefault()),
		TokenURL:  fmt.Sprintf("%s/oauth2/ssh/token", eg.EnvAPIHostDefault()),
		AuthStyle: oauth2.AuthStyleInHeader,
	}
}

func EndpointCompute() string {
	return fmt.Sprintf("%s/c/authz/", eg.EnvAPIHostDefault())
}

func OAuth2SSHConfig(signer ssh.Signer, otp string, endpoint oauth2.Endpoint) oauth2.Config {
	return oauth2.Config{
		ClientID:     ssh.FingerprintSHA256(signer.PublicKey()),
		ClientSecret: otp,
		Endpoint:     endpoint,
	}
}

func OAuth2SSHToken(ctx context.Context, signer ssh.Signer, endpoint oauth2.Endpoint) (cfg oauth2.Config, tok *oauth2.Token, err error) {
	var (
		sig *ssh.Signature
	)

	password := uuid.Must(uuid.NewV4())

	cfg = OAuth2SSHConfig(signer, password.String(), endpoint)
	if sig, err = signer.Sign(rand.Reader, password.Bytes()); err != nil {
		return cfg, nil, err
	}

	encodedsig := base64.RawURLEncoding.EncodeToString(ssh.Marshal(sig))

	tok, err = cfg.PasswordCredentialsToken(ctx, cfg.ClientID, encodedsig)
	return cfg, tok, err
}

func ExchangeAuthed(ctx context.Context, chttp *http.Client, endpoint string, authed *Authed) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := httpx.AsError(chttp.Do(req))
	if err != nil {
		return err
	}
	defer httpx.AutoClose(resp)

	if err = json.NewDecoder(resp.Body).Decode(&authed); err != nil {
		return err
	}

	return nil
}

func AutoTokenState(signer ssh.Signer) (encoded string, err error) {
	type reqstate struct {
		ID        string `json:"id"`
		PublicKey []byte `json:"pkey"`
	}

	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	rawstate := reqstate{
		ID:        id.String(),
		PublicKey: signer.PublicKey().Marshal(),
	}

	if encoded, err = jwtx.EncodeJSON(rawstate); err != nil {
		return "", errorsx.Wrap(err, "unable to encode state")
	}

	return encoded, nil
}

func AutoRefreshTokenState(ctx context.Context, signer ssh.Signer, state string) (_ *oauth2.Token, err error) {
	var (
		ok        bool
		chttp     *http.Client
		exchanged jwtx.AuthResponse
		token     *oauth2.Token
	)

	if t, err := ReadRefreshToken(); err == nil {
		return t, nil
	}

	if chttp, ok = ctx.Value(oauth2.HTTPClient).(*http.Client); !ok {
		return nil, fmt.Errorf("missing http client from context")
	}

	cfg := OAuth2SSHConfig(signer, "", EndpointSSHAuth())

	authzuri := cfg.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
	)

	if exchanged, err = jwtx.RetrieveAuthCode(ctx, chttp, authzuri); err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve auth code")
	}

	if exchanged.State != state {
		return nil, fmt.Errorf("mismatch oauth state sent: '%s' retrieved: '%s'", state, exchanged.State)
	}

	if token, err = cfg.Exchange(ctx, exchanged.Code, oauth2.AccessTypeOffline); err != nil {
		return nil, errorsx.Wrap(err, "failed to exchange code of oauth2 token")
	}

	if err = WriteRefreshToken(token.RefreshToken); err != nil {
		return nil, errorsx.Wrap(err, "unable to write token to disk")
	}

	return token, nil
}

func OAuth2SSHHTTPClient(ctx context.Context, signer ssh.Signer, endpoint oauth2.Endpoint) (_ *http.Client, err error) {
	token, err := AutoRefreshToken(ctx, signer)
	if err != nil {
		return nil, err
	}

	cfg := OAuth2SSHConfig(signer, "", endpoint)
	return cfg.Client(ctx, token), nil
}

func AutoRefreshToken(ctx context.Context, signer ssh.Signer) (_ *oauth2.Token, err error) {
	var (
		state string
	)

	if t, err := ReadRefreshToken(); err == nil {
		return t, nil
	}

	if state, err = AutoTokenState(signer); err != nil {
		return nil, err
	}

	return AutoRefreshTokenState(ctx, signer, state)
}

func WriteRefreshToken(token string) (err error) {
	basedir := userx.DefaultCacheDirectory()
	path := filepath.Join(basedir, "session.token")
	if err = os.MkdirAll(basedir, 0700); err != nil {
		return err
	}

	if err = os.Setenv("EG_SESSION_TOKEN", token); err != nil {
		return err
	}

	if err = os.WriteFile(path, []byte(token), 0600); err != nil {
		return err
	}

	return nil
}

func ReadRefreshToken() (_ *oauth2.Token, err error) {
	raw, ok := os.LookupEnv("EG_SESSION_TOKEN")
	if !ok {
		return nil, fmt.Errorf("session token missing")
	}

	return &oauth2.Token{
		TokenType:    "BEARER",
		RefreshToken: string(raw),
	}, err
}

func BearerAuthorization(req *http.Request, token string) {
	req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", token))
}

func Session(ctx context.Context, c *http.Client, bearer string) (_ *Current, err error) {
	var (
		session Current
		req     *http.Request
	)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/authn/current", eg.EnvAPIHostDefault()), nil)
	if err != nil {
		return nil, err
	}
	BearerAuthorization(req, bearer)
	resp, err := httpx.AsError(c.Do(req))
	if err != nil {
		return nil, err
	}
	defer httpx.AutoClose(resp)

	if err = json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}

	return &session, nil
}
