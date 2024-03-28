package accountcmds

import (
	"context"
	"errors"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/gofrs/uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type Register struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Name       string `name:"name" help:"name of the account to create" default:"${vars_user_name}"`
	Email      string `name:"email" help:"business contact email" required:""`
}

func (t Register) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
	type reqstate struct {
		ID        string `json:"id"`
		PublicKey []byte `json:"pkey"`
		Email     string `json:"email"`
		Display   string `json:"display"`
	}

	var (
		signer    ssh.Signer
		authed    authn.Authed
		encoded   string
		exchanged jwtx.AuthResponse
		token     *oauth2.Token
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	password := uuid.Must(uuid.NewV4())
	rawstate := reqstate{
		ID:        md5x.String(password.String()),
		PublicKey: signer.PublicKey().Marshal(),
		Email:     t.Email,
		Display:   t.Name,
	}

	if encoded, err = jwtx.EncodeJSON(rawstate); err != nil {
		return errorsx.Wrap(err, "unable to encode state")
	}

	chttp := tlscfg.DefaultClient()
	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, chttp)
	cfg := authn.OAuth2SSHConfig(signer, password.String(), authn.EndpointSSHAuth())

	authzuri := cfg.AuthCodeURL(
		encoded,
		oauth2.AccessTypeOffline,
	)

	if exchanged, err = jwtx.RetrieveAuthCode(ctx, chttp, authzuri); err != nil {
		return errorsx.Wrap(err, "unable to retrieve auth code")
	}

	if exchanged.State != encoded {
		return errorsx.Wrap(err, "mismatch oauth state")
	}

	if token, err = cfg.Exchange(ctx, exchanged.Code, oauth2.AccessTypeOffline); err != nil {
		return errorsx.Wrap(err, "failed to exchange code of oauth2 token")
	}

	if err = authn.WriteRefreshToken(token.RefreshToken); err != nil {
		return errorsx.Wrap(err, "unable to write token to disk")
	}

	// debugx.Println("token received", spew.Sdump(token))
	if err = loginssh(ctx, cfg.Client(ctx, token), &authed); err != nil {
		return err
	}

	// log.Println("authed", spew.Sdump(&authed))

	switch len(authed.Profiles) {
	case 0:
		return signup(ctx, chttp, &authed)
	case 1:
		return session(ctx, chttp, authed.Profiles[0])
	default:
		return errorsx.Notification(errors.New("you've already registered an account; multiple account support will be implemented in the future"))
	}
}
