// Package cmdapt manages private apt repositories hosted by eg, served from
// https://<account id>.<apt domain>/<repository>.
package cmdapt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/egdaemon/eg/apt"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdauthz"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
)

type Cmd struct {
	Repo      Repo      `cmd:"" name:"repo" help:"create, inspect and configure apt repositories"`
	Upload    Upload    `cmd:"" name:"upload" help:"upload debian packages to an apt repository"`
	Publish   Publish   `cmd:"" name:"publish" help:"publish the packages uploaded to an apt repository"`
	Transport Transport `cmd:"" name:"transport" hidden:"true" help:"apt acquire method for egs:// uris, run by apt via /usr/lib/apt/methods/egs"`
}

type Repo struct {
	Create  Create  `cmd:"" name:"create" help:"create an apt repository"`
	Show    Show    `cmd:"" name:"show" help:"show an apt repository"`
	Set     Set     `cmd:"" name:"set" help:"change the settings of an apt repository, it is not republished"`
	Key     Key     `cmd:"" name:"key" help:"fetch the signing public key of an apt repository"`
	Sources Sources `cmd:"" name:"sources" help:"print the apt sources line of a repository and the commands to install its key"`
}

// target the repository commands act on.
type target struct {
	cmdauthz.Identity `embed:""`
	AptDomain         string `name:"apt-domain" help:"domain of the apt repository service" default:"${vars_apt_domain}" hidden:"true"`
	Name              string `arg:"" name:"repository" help:"name of the repository"`
}

// resolve the account and an http client for the repository authorized with the endpoint's
// tokens, see authn.EndpointCompute and authn.EndpointMetaAuthz.
func (t target) resolve(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, endpoint string) (aid string, _ apt.Client, err error) {
	if err = apt.ValidName(t.Name); err != nil {
		return "", apt.Client{}, err
	}

	signer, err := t.Signer()
	if err != nil {
		return "", apt.Client{}, err
	}

	if aid, err = t.Account(gctx, tlsc, ident, signer); err != nil {
		return "", apt.Client{}, err
	}

	c := cmdauthz.Client(gctx, tlsc.DefaultClient(), signer, endpoint, aid)
	c.CheckRedirect = apt.SameHostRedirects

	return aid, apt.New(c, apt.URL("https", t.AptDomain, aid, t.Name)), nil
}

// anonymous client for the repository, for the endpoints that need no authorization (key.asc).
func (t target) anonymous(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (apt.Client, error) {
	if err := apt.ValidName(t.Name); err != nil {
		return apt.Client{}, err
	}

	aid := gctx.AccountID
	if aid == "" {
		signer, err := t.Signer()
		if err != nil {
			return apt.Client{}, err
		}

		if aid, err = t.Account(gctx, tlsc, ident, signer); err != nil {
			return apt.Client{}, err
		}
	}

	return apt.New(tlsc.DefaultClient(), apt.URL("https", t.AptDomain, aid, t.Name)), nil
}

func (t target) ci(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (apt.Client, error) {
	_, c, err := t.resolve(gctx, tlsc, ident, authn.EndpointCompute())
	return c, err
}

func (t target) meta(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (apt.Client, error) {
	_, c, err := t.resolve(gctx, tlsc, ident, authn.EndpointMetaAuthz())
	return c, err
}

// settings shared by create and set.
type settings struct {
	Architectures []string `name:"arch" help:"architectures of the repository, defaults to amd64" sep:","`
	Suite         *string  `name:"suite" help:"suite of the repository, defaults to stable"`
	Codename      *string  `name:"codename" help:"codename of the repository, defaults to stable"`
	Component     *string  `name:"component" help:"component of the repository, defaults to main"`
	Origin        *string  `name:"origin" help:"origin field of the Release file"`
	Label         *string  `name:"label" help:"label field of the Release file"`
	Public        *bool    `name:"public" help:"serve the repository to anonymous clients, requires a paid account" negatable:""`
}

func (t settings) settings() apt.Settings {
	return apt.Settings{
		Architectures: t.Architectures,
		Suite:         t.Suite,
		Codename:      t.Codename,
		Component:     t.Component,
		Origin:        t.Origin,
		Label:         t.Label,
		Public:        t.Public,
	}
}

func duration(d *time.Duration) *string {
	if d == nil {
		return nil
	}

	s := d.String()
	return &s
}

// printjson writes the json indented, the repository json is printed as the server returns it.
func printjson(dst io.Writer, raw json.RawMessage) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		_, err = fmt.Fprintln(dst, string(raw))
		return err
	}

	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(dst, string(encoded))
	return err
}

func explain(err error, action string) error {
	switch {
	case err == nil:
		return nil
	case apt.Status(err, http.StatusUnauthorized), apt.Status(err, http.StatusForbidden):
		return fmt.Errorf("unable to %s, the profile is not authorized: %w", action, err)
	case apt.Status(err, http.StatusPaymentRequired):
		return fmt.Errorf("unable to %s, public repositories require a paid account: %w", action, err)
	case apt.Status(err, http.StatusNotFound):
		return fmt.Errorf("unable to %s, repository not found: %w", action, err)
	default:
		return fmt.Errorf("unable to %s: %w", action, err)
	}
}
