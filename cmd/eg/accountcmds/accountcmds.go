package accountcmds

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/james-lawrence/eg"
	"github.com/james-lawrence/eg/authn"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/httpx"
)

func signup(ctx context.Context, signupToken string) (err error) {
	var (
		session authn.Current
		req     *http.Request
	)
	ctransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	chttp := &http.Client{Transport: ctransport, Timeout: 10 * time.Second}

	req, err = http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/authn/signup", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", signupToken)

	resp3, err := httpx.AsError(chttp.Do(req))
	if err != nil {
		return err
	}
	defer httpx.AutoClose(resp3)

	if err = json.NewDecoder(resp3.Body).Decode(&session); err != nil {
		return err
	}

	log.Println("logged in as", session.Profile.Display, "-", session.Profile.Email)
	log.Println("account", session.Account.Display)
	return authn.WriteSessionToken(session.Token)
}

func login(ctx context.Context, authed *authn.Authn) (err error) {
	var (
		session authn.Current
		req     *http.Request
	)
	ctransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	chttp := &http.Client{Transport: ctransport, Timeout: 10 * time.Second}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/authn/current", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", authed.Token)

	resp, err := httpx.AsError(chttp.Do(req))
	if err != nil {
		return err
	}
	defer httpx.AutoClose(resp)

	if err = json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return err
	}

	log.Println("logged in as", session.Profile.Display, "-", session.Profile.Email)
	log.Println("account", session.Account.Display)
	return authn.WriteSessionToken(session.Token)
}
