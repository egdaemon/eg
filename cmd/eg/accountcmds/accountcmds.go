package accountcmds

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/james-lawrence/eg"
	"github.com/james-lawrence/eg/authn"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/httpx"
	"github.com/james-lawrence/eg/internal/stringsx"
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
	return authn.WriteSessionToken(session.Token)
}

func login(ctx context.Context, chttp *http.Client, authed *authn.Authn) (err error) {
	var (
		session authn.Current
		req     *http.Request
	)

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/authn/current", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), nil)
	if err != nil {
		return err
	}
	authn.BearerAuthorization(req, authed.Token)

	resp, err := httpx.AsError(chttp.Do(req))
	if err != nil {
		return err
	}
	defer httpx.AutoClose(resp)

	if err = json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return err
	}

	log.Println("logged in as", session.Profile.Display, "-", session.Profile.Id)
	log.Println("account", stringsx.DefaultIfBlank(session.Account.Display, session.Profile.AccountId))
	return authn.WriteSessionToken(session.Token)
}

func otp(ctx context.Context, chttp *http.Client, authed *authn.Authn) (err error) {
	fmt.Printf("%s?lt=%s\n", envx.String(eg.EnvEGConsoleHostDefault, eg.EnvEGConsoleHost), authed.Token)
	return nil
}
