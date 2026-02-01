package accountcmds

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os/exec"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/crypto/ssh"
)

type Signup struct {
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Seed        string `name:"seed" help:"seed for generating determistic credentials, useful for ci/cd platforms" default:"${vars_entropy_seed}"`
	Endpoint    string `name:"endpoint" help:"specify the endpoint to connect to" default:"${vars_console_endpoint}" hidden:"true"`
	Account     string `name:"account" help:"optional name of the account you want to register with can be found at https://console.egdaemon.com/s/settings"`
	AutoBrowser bool   `name:"browser" help:"automatically open browser if possible" default:"false"`
}

func (t Signup) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
	var (
		signer ssh.Signer
		uri    *url.URL
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGenSeeded(t.Seed), t.SSHKeyPath); err != nil {
		return err
	}

	if uri, err = url.Parse(fmt.Sprintf("%s/me/identity/ssh", t.Endpoint)); err != nil {
		return errorsx.Wrapf(err, "invalid endpoint url %s", t.Endpoint)
	}

	if !stringsx.Blank(t.Account) {
		uri.Host = fmt.Sprintf("%s.%s", t.Account, uri.Host)
	}
	q := uri.Query()
	q.Add("pubkey", base64.URLEncoding.EncodeToString(signer.PublicKey().Marshal()))

	uri.RawQuery = q.Encode()

	fmt.Println("open to register:", uri.String())

	if t.AutoBrowser {
		errorsx.Log(errorsx.Wrap(exec.CommandContext(gctx.Context, "xdg-open", uri.String()).Run(), "unable to automatically open browser"))
	}

	return nil
}
