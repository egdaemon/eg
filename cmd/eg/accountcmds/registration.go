package accountcmds

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg"
	"github.com/james-lawrence/eg/authn"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/httpx"
	"github.com/james-lawrence/eg/internal/sshx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type Register struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Name       string `name:"name" help:"name of the account to create" default:"${vars_user_name}"`
	Email      string `email:"email" help:"business contact email" required:""`
}

func (t Register) Run(gctx *cmdopts.Global) (err error) {
	var (
		signer ssh.Signer
		sig    *ssh.Signature
		authed authn.Authed
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	password := uuid.Must(uuid.NewV4())
	encoded := base64.RawURLEncoding.EncodeToString(signer.PublicKey().Marshal())

	ctransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	chttp := &http.Client{Transport: ctransport, Timeout: 10 * time.Second}

	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, chttp)
	cfg := authn.OAuth2SSHConfig(signer, password.String())

	authzuri := cfg.AuthCodeURL(
		encoded,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("email", t.Email),
		oauth2.SetAuthURLParam("description", t.Name),
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
	resp2, err := httpx.AsError(authzc.Do(req))
	if err != nil {
		return err
	}
	defer httpx.AutoClose(resp2)

	if err = json.NewDecoder(resp2.Body).Decode(&authed); err != nil {
		return err
	}

	signup := func() error {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/authn/signup", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), nil)
		if err != nil {
			return err
		}
		req.Header.Add("Authorization", authed.SignupToken)
		resp3, err := httpx.AsError(chttp.Do(req))
		if err != nil {
			return err
		}
		defer httpx.AutoClose(resp3)

		if err = json.NewDecoder(resp3.Body).Decode(&authed); err != nil {
			return err
		}

		return nil
	}

	switch len(authed.Profiles) {
	case 0:
		return signup()
	default:
		return errorsx.Notification(errors.New("you've already registered an account; multiple account support will be implemented in the future"))
	}
}
