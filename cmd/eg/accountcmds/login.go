package accountcmds

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

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/gofrs/uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type Login struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t Login) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
	type reqstate struct {
		PublicKey []byte `json:"pkey"`
		Email     string `json:"email"`
		Display   string `json:"display"`
	}

	var (
		signer  ssh.Signer
		sig     *ssh.Signature
		authed  authn.Authed
		encoded string
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	password := uuid.Must(uuid.NewV4())

	rawstate := reqstate{
		PublicKey: signer.PublicKey().Marshal(),
	}

	if encoded, err = jwtx.EncodeJSON(rawstate); err != nil {
		return err
	}

	chttp := tlscfg.DefaultClient()
	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, chttp)
	cfg := authn.OAuth2SSHConfig(signer, password.String())

	authzuri := cfg.AuthCodeURL(
		encoded,
		oauth2.AccessTypeOffline,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authzuri, nil)
	if err != nil {
		return err
	}
	resp, err := httpx.AsError(chttp.Do(req))
	if err != nil {
		return err
	}
	defer httpx.AutoClose(resp)

	if sig, err = signer.Sign(rand.Reader, password.Bytes()); err != nil {
		return err
	}

	encodedsig := base64.RawURLEncoding.EncodeToString(ssh.Marshal(sig))

	token, err := cfg.PasswordCredentialsToken(ctx, cfg.ClientID, encodedsig)
	if err != nil {
		return err
	}

	authzc := cfg.Client(ctx, token)

	req, err = http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/authn/ssh", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), nil)
	if err != nil {
		return err
	}
	resp3, err := httpx.AsError(authzc.Do(req))
	if err != nil {
		return err
	}
	defer httpx.AutoClose(resp3)

	if err = json.NewDecoder(resp3.Body).Decode(&authed); err != nil {
		return err
	}

	log.Println("authed", spew.Sdump(&authed))

	switch len(authed.Profiles) {
	case 0:
		return errorsx.Notification(fmt.Errorf(
			"the ssh key (%s) is not associated with any profiles, unable to login. associate the key with a profile or use the `%s register --help` command to setup a new account",
			t.SSHKeyPath,
			stringsx.First(os.Args...),
		))
	case 1:
		return login(ctx, chttp, authed.Profiles[0])
	default:
		return errorsx.Notification(errors.New("you've already registered an account; multiple account support will be implemented in the future"))
	}
}
