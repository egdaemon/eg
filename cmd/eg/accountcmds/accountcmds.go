package accountcmds

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/stringsx"
)

func signup(ctx context.Context, chttp *http.Client, authed *authn.Authed) (err error) {
	var (
		session authn.Current
		req     *http.Request
	)

	req, err = http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/authn/signup", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), nil)
	if err != nil {
		return errorsx.Wrap(err, "signup request")
	}
	authn.BearerAuthorization(req, authed.SignupToken)

	resp3, err := httpx.AsError(chttp.Do(req))
	if err != nil {
		return errorsx.Wrap(err, "signup failed")
	}
	defer httpx.AutoClose(resp3)

	if err = json.NewDecoder(resp3.Body).Decode(&session); err != nil {
		return errorsx.Wrap(err, "signup bad response")
	}

	log.Println("logged in as", session.Profile.Display, "-", session.Profile.Id)
	log.Println("account", stringsx.DefaultIfBlank(session.Account.Display, session.Profile.AccountId))
	return nil
}

func loginssh(ctx context.Context, chttp *http.Client, authed *authn.Authed) (err error) {
	return authn.ExchangeAuthed(ctx, chttp, fmt.Sprintf("%s/authn/ssh", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), authed)
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
	fmt.Printf("%s?lt=%s\n", envx.String(eg.EnvEGConsoleHostDefault, eg.EnvEGConsoleHost), authed.Token)
	return nil
}
