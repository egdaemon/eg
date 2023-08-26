package actl

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"log"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg/authn"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/sshx"
	"github.com/james-lawrence/eg/registration"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type AuthorizeAgent struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t AuthorizeAgent) Run(gctx *cmdopts.Global) (err error) {
	var (
		signer ssh.Signer
		sig    *ssh.Signature
		sresp  *registration.RegistrationSearchResponse
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	otp := uuid.Must(uuid.NewV4())

	ctransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	chttp := &http.Client{Transport: ctransport, Timeout: 10 * time.Second}

	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, chttp)
	cfg := authn.OAuth2SSHConfig(signer, otp.String())

	if sig, err = signer.Sign(rand.Reader, otp.Bytes()); err != nil {
		return err
	}

	token, err := cfg.PasswordCredentialsToken(ctx, cfg.ClientID, base64.RawURLEncoding.EncodeToString(ssh.Marshal(sig)))
	if err != nil {
		return err
	}

	httpc := cfg.Client(ctx, token)
	// resp, err := httpc.Post(fmt.Sprintf("%s/authn/ssh", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), "application/json", nil)
	// if err != nil {
	// 	return err
	// }
	// if decoded, err := httputil.DumpResponse(resp, true); err == nil {
	// 	log.Println("DERP", string(decoded))
	// }

	rc := registration.NewRegistrationClient(httpc)
	if sresp, err = rc.Search(ctx, &registration.RegistrationSearchRequest{}); err != nil {
		return err
	}

	log.Println("Search Response", spew.Sdump(sresp))
	return nil
}
