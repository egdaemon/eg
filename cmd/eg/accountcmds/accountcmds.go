package accountcmds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"
)

func signup(ctx context.Context, chttp *http.Client, authed *authn.Authed) (err error) {
	return postsession(ctx, chttp, authn.EndpointSignup(), authed.SignupToken)
}

// sshsession signs the request with the ssh key, the server verifies it against the public key it recorded
// for the key. the issuer is the id of the sso domain to register with, blank to signup for a new account.
func sshsession(ctx context.Context, chttp *http.Client, signer ssh.Signer, endpoint, issuer string) (err error) {
	token, err := jwt.NewWithClaims(
		jwtx.NewSSHSigner(),
		jwtx.NewJWTClaims(identity.ID(signer), jwtx.ClaimsOptionIssuer(issuer)),
	).SignedString(signer)
	if err != nil {
		return errorsx.Wrap(err, "unable to sign request")
	}

	return postsession(ctx, chttp, endpoint, token)
}

func postsession(ctx context.Context, chttp *http.Client, endpoint string, token string) (err error) {
	var (
		session authn.Current
		req     *http.Request
	)

	req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return errorsx.Wrap(err, "session request")
	}
	authn.BearerAuthorization(req, token)

	resp3, err := httpx.AsError(chttp.Do(req))
	if err != nil {
		return errorsx.Wrapf(err, "session request to %s failed", endpoint)
	}
	defer httpx.AutoClose(resp3)

	if err = json.NewDecoder(resp3.Body).Decode(&session); err != nil {
		return errorsx.Wrap(err, "session bad response")
	}

	log.Println("logged in as", session.Profile.Display, "-", session.Profile.Id)
	log.Println("account", stringsx.DefaultIfBlank(session.Account.Display, session.Profile.AccountId))
	return nil
}

func loginssh(ctx context.Context, chttp *http.Client, options io.Reader, authed *authn.Authed) (err error) {
	return authn.ExchangeAuthed(ctx, chttp, fmt.Sprintf("%s/authn/ssh", eg.EnvAPIHostDefault()), options, authed)
}

func session(ctx context.Context, chttp *http.Client, authed *authn.Authn) (err error) {
	session, err := authn.Session(ctx, chttp, authed.Token)
	if err != nil {
		return err
	}

	log.Println("logged in as", session.Profile.Display, "-", session.Profile.Id)
	log.Println("account", stringsx.DefaultIfBlank(session.Account.Display, session.Profile.AccountId))
	return nil
}

func otp(ctx context.Context, chttp *http.Client, authed *authn.Authn) (err error) {
	fmt.Printf("%s?lt=%s\n", eg.EnvConsoleHostDefault(), authed.Token)
	return nil
}
