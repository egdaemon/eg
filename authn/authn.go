package authn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/gofrs/uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

func OAuth2SSHConfig(signer ssh.Signer, otp string) oauth2.Config {
	return oauth2.Config{
		ClientID:     ssh.FingerprintSHA256(signer.PublicKey()),
		ClientSecret: otp,
		Endpoint: oauth2.Endpoint{
			AuthURL:   fmt.Sprintf("%s/oauth2/ssh/auth", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)),
			TokenURL:  fmt.Sprintf("%s/oauth2/ssh/token", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)),
			AuthStyle: oauth2.AuthStyleInHeader,
		},
	}
}

func OAuth2SSHToken(ctx context.Context, signer ssh.Signer) (cfg oauth2.Config, tok *oauth2.Token, err error) {
	var (
		sig *ssh.Signature
	)

	password := uuid.Must(uuid.NewV4())

	cfg = OAuth2SSHConfig(signer, password.String())
	if sig, err = signer.Sign(rand.Reader, password.Bytes()); err != nil {
		return cfg, nil, err
	}

	encodedsig := base64.RawURLEncoding.EncodeToString(ssh.Marshal(sig))
	tok, err = cfg.PasswordCredentialsToken(ctx, cfg.ClientID, encodedsig)
	return cfg, tok, err
}

func OAuth2SSHHTTPClient(ctx context.Context, signer ssh.Signer) (_ *http.Client, err error) {
	cfg, token, err := OAuth2SSHToken(ctx, signer)
	if err != nil {
		return nil, err
	}
	return cfg.Client(ctx, token), nil
}

func WriteSessionToken(token string) (err error) {
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

func ReadSessionToken() (_ string, err error) {
	path := filepath.Join(userx.DefaultCacheDirectory(), "session.token")
	raw, err := os.ReadFile(path)
	return string(raw), err
}

func BearerAuthorization(req *http.Request, token string) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
}
