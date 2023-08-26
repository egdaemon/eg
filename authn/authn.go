package authn

import (
	"fmt"

	"github.com/james-lawrence/eg"
	"github.com/james-lawrence/eg/internal/envx"
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
