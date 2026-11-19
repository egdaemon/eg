package cmdapt

import (
	"fmt"
	"net/http"
	"os"

	"github.com/egdaemon/eg/apt"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdauthz"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
)

// Transport is the apt acquire method for egs:// uris. apt runs /usr/lib/apt/methods/egs as root,
// which execs this command; it speaks the method protocol on stdin and stdout, so nothing else may
// be written to stdout. the account comes from the host of each uri, the ssh identity is the one of
// the user apt runs the method as, see --sshkeypath and EG_SSH_KEY_PATH.
type Transport struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t Transport) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	m := &apt.Method{
		Client: func(account string) (*http.Client, error) {
			// never generate an identity: a new key is not authorized for anything.
			if _, err := os.Stat(t.SSHKeyPath); err != nil {
				return nil, fmt.Errorf("eg ssh identity missing at %s, set EG_SSH_KEY_PATH to the identity authorized for the account: %w", t.SSHKeyPath, err)
			}

			signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
			if err != nil {
				return nil, errorsx.Wrap(err, "unable to load ssh identity")
			}

			c := cmdauthz.Client(gctx, tlsc.DefaultClient(), signer, authn.EndpointCompute(), account)
			c.CheckRedirect = apt.SameHostRedirects
			return c, nil
		},
	}

	return m.Run(gctx.Context, os.Stdin, os.Stdout)
}
