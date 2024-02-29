package actl

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/registration"
	"github.com/gofrs/uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type AuthorizeAgent struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	ID         string `name:"id" help:"grant authorization to compute" required:""`
}

func (t AuthorizeAgent) Run(gctx *cmdopts.Global) (err error) {
	var (
		at     string
		signer ssh.Signer
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	otp := uuid.Must(uuid.NewV4())

	ctransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	chttp := &http.Client{Transport: ctransport, Timeout: 10 * time.Second}
	chttp = httpx.DebugClient(chttp)

	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, chttp)
	cfg := authn.OAuth2SSHConfig(signer, otp.String())

	if at, err = authn.ReadSessionToken(); err != nil {
		return err
	}

	httpc := cfg.Client(ctx, &oauth2.Token{AccessToken: at})
	rc := registration.NewRegistrationClient(httpc)
	if _, err = rc.Grant(ctx, &registration.RegistrationGrantRequest{Registration: &registration.Registration{Id: t.ID}}); err != nil {
		return err
	}

	return nil
}
