package authn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/gofrs/uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

func EndpointSSHAuth() oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:   fmt.Sprintf("%s/oauth2/ssh/auth", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)),
		TokenURL:  fmt.Sprintf("%s/oauth2/ssh/token", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)),
		AuthStyle: oauth2.AuthStyleInHeader,
	}
}

func EndpointCompute() string {
	return fmt.Sprintf("%s/c/authz/token", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost))
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
func NewAuthzTokenSource(c *http.Client, signer ssh.Signer, endpoint string) TokenSourceFromEndpoint {
	return TokenSourceFromEndpoint{c: c, signer: signer, endpoint: endpoint}
}

type TokenSourceFromEndpoint struct {
	c        *http.Client
	endpoint string
	signer   ssh.Signer
}

func (t TokenSourceFromEndpoint) Token() (_ *oauth2.Token, err error) {
	type bearer struct {
		Bearer string `json:"bearer"`
	}
	type msg struct {
		Token bearer `json:"token"`
	}
	var (
		authed Authed
		token  msg
	)
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	cfg := OAuth2SSHConfig(t.signer, "", EndpointSSHAuth())
	refreshtoken, err := ReadRefreshToken()
	if err != nil {
		return nil, err
	}

	chttp := cfg.Client(context.WithValue(ctx, oauth2.HTTPClient, t.c), refreshtoken)

	if err = ExchangeAuthed(ctx, chttp, fmt.Sprintf("%s/authn/ssh", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), &authed); err != nil {
		return nil, err
	}

	if len(authed.Profiles) != 1 {
		return nil, errors.New("too many profiles")
	}

	session, err := Session(ctx, t.c, authed.Profiles[0].Token)
	if err != nil {
		return nil, err
	}

	chttp = cfg.Client(context.WithValue(ctx, oauth2.HTTPClient, t.c), &oauth2.Token{
		TokenType:   "BEARER",
		AccessToken: session.Token,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpx.AsError(httpx.DebugClient(chttp).Do(req))
	if err != nil {
		return nil, err
	}
	defer httpx.AutoClose(resp)

	if err = json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	return &oauth2.Token{TokenType: "BEARER", AccessToken: token.Token.Bearer}, nil
}

func OAuth2SSHHTTPClient(ctx context.Context, signer ssh.Signer, endpoint oauth2.Endpoint) (_ *http.Client, err error) {
	cfg := OAuth2SSHConfig(signer, "", endpoint)

	token, err := ReadRefreshToken()
	if err != nil {
		return nil, err
	}

	return cfg.Client(ctx, token), nil
}

func WriteRefreshToken(token string) (err error) {
	basedir := userx.DefaultCacheDirectory()
	path := filepath.Join(basedir, "session.token")
	if err = os.MkdirAll(basedir, 0700); err != nil {
		return err
	}

	if err = os.WriteFile(path, []byte(token), 0600); err != nil {
		return err
	}

	return nil
}

func ReadRefreshToken() (_ *oauth2.Token, err error) {
	path := filepath.Join(userx.DefaultCacheDirectory(), "session.token")
	raw, err := os.ReadFile(path)
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
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/authn/current", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), nil)
	if err != nil {
		log.Println("CHECKPOINT")
		return nil, err
	}
	BearerAuthorization(req, bearer)
	resp, err := httpx.AsError(c.Do(req))
	if err != nil {
		return nil, err
	}
	defer httpx.AutoClose(resp)

	if err = json.NewDecoder(resp.Body).Decode(&session); err != nil {
		log.Println("CHECKPOINT")
		return nil, err
	}

	return &session, nil
}
