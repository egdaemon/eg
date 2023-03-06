package actl

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/sshx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type AuthorizeAgent struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

// allowing the control plane to interogate
func (t AuthorizeAgent) Run(gctx *cmdopts.Global) (err error) {
	var (
		signer ssh.Signer
		sig    *ssh.Signature
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	password := uuid.Must(uuid.NewV4())

	ctransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	chttp := &http.Client{Transport: ctransport, Timeout: 10 * time.Second}

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, chttp)
	cfg := oauth2.Config{
		ClientID:     ssh.FingerprintSHA256(signer.PublicKey()),
		ClientSecret: password.String(),
		Endpoint: oauth2.Endpoint{
			AuthURL:   fmt.Sprintf("%s/oauth2/ssh/auth", envx.String("https://localhost:3001", eg.EnvEGAPIHost)),
			TokenURL:  fmt.Sprintf("%s/oauth2/ssh/token", envx.String("https://localhost:3001", eg.EnvEGAPIHost)),
			AuthStyle: oauth2.AuthStyleInHeader,
		},
	}

	if sig, err = signer.Sign(rand.Reader, uuid.FromStringOrNil(cfg.ClientSecret).Bytes()); err != nil {
		return err
	}

	token, err := cfg.PasswordCredentialsToken(ctx, cfg.ClientID, base64.RawURLEncoding.EncodeToString(ssh.Marshal(sig)))
	if err != nil {
		return err
	}

	httpc := cfg.Client(ctx, token)
	resp, err := httpc.Post(fmt.Sprintf("%s/authn/ssh", envx.String("https://localhost:3001", eg.EnvEGAPIHost)), "application/json", nil)
	if err != nil {
		return err
	}
	if decoded, err := httputil.DumpResponse(resp, true); err == nil {
		log.Println("DERP", string(decoded))
	}
	return nil
}
