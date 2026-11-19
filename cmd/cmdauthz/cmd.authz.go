// Package cmdauthz prints authorization tokens for the eg api, for tools that cannot use the
// ssh identity directly, for example:
//
//	debmirror --user=token --passwd="$(eg authz ci token)" ...
package cmdauthz

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type Cmd struct {
	Meta Meta `cmd:"" name:"meta" help:"account management authorization (usermanagement), used to create and change apt repositories"`
	CI   CI   `cmd:"" name:"ci" help:"ci authorization (repository_read/repository_modify), used to read, upload to and publish apt repositories"`
}

type Meta struct {
	Token MetaToken `cmd:"" name:"token" help:"print a meta authorization token to stdout"`
}

type CI struct {
	Token CIToken `cmd:"" name:"token" help:"print a ci authorization token to stdout"`
}

// Identity selects the ssh identity and account requests are authorized as.
type Identity struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

// Signer the ssh identity, it is generated when missing.
func (t Identity) Signer() (ssh.Signer, error) {
	return sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
}

// Account resolves the account requests are authorized for.
func (t Identity) Account(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, signer ssh.Signer) (string, error) {
	return cmdidentity.AccountID(ident.With(identity.QueryEnvironment(gctx.AccountID, gctx.ProfileID, t.VcsURI)).Resolve(tlsc.DefaultClient(), signer))
}

// TokenSource of the endpoint's authorization for the account, see authn.EndpointCompute and
// authn.EndpointMetaAuthz.
func TokenSource(c *http.Client, signer ssh.Signer, endpoint string, aid string, pid string) oauth2.TokenSource {
	return oauth2.ReuseTokenSource(nil, compute.NewAuthzTokenSource(c, signer, endpoint, aid, pid))
}

// Client authorized with the endpoint's tokens for the account.
func Client(gctx *cmdopts.Global, c *http.Client, signer ssh.Signer, endpoint string, aid string) *http.Client {
	return authn.New(gctx.Context, c, signer, aid, gctx.ProfileID, compute.NewAuthzTokenSource, authn.OptionEndpoint(endpoint))
}

// MetaToken prints a meta authorization token.
type MetaToken struct {
	Identity `embed:""`
}

func (t MetaToken) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) error {
	return t.Identity.print(os.Stdout, gctx, tlsc, ident, authn.EndpointMetaAuthz())
}

// CIToken prints a ci authorization token.
type CIToken struct {
	Identity `embed:""`
}

func (t CIToken) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) error {
	return t.Identity.print(os.Stdout, gctx, tlsc, ident, authn.EndpointCompute())
}

func (t Identity) print(dst io.Writer, gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, endpoint string) error {
	signer, err := t.Signer()
	if err != nil {
		return err
	}

	aid, err := t.Account(gctx, tlsc, ident, signer)
	if err != nil {
		return err
	}

	tok, err := TokenSource(tlsc.DefaultClient(), signer, endpoint, aid, gctx.ProfileID).Token()
	if err != nil {
		return errorsx.Wrap(err, "unable to retrieve token")
	}

	_, err = fmt.Fprintln(dst, tok.AccessToken)
	return err
}
