package authn

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/userx"
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
