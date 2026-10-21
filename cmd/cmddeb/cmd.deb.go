// Package cmddeb manages deb namespaces, apt repositories hosted by eg.
package cmddeb

import (
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/debrepo"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
)

type Cmd struct {
	Create Create `cmd:"" name:"create" help:"create a namespace"`
	List   List   `cmd:"" name:"ls" help:"list namespaces"`
	Show   Show   `cmd:"" name:"show" help:"display a namespace and how to install from it"`
	Edit   Edit   `cmd:"" name:"edit" help:"edit a namespace"`
	Remove Remove `cmd:"" name:"rm" help:"remove a namespace and its packages"`
	Token  Token  `cmd:"" name:"token" help:"print a short lived token for publishing (EG_DEB_TOKEN) or apt authentication"`
}

type auth struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t auth) client(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (*http.Client, error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return nil, err
	}

	return authn.New(gctx.Context, tlsc.DefaultClient(), signer, gctx.AccountID, compute.NewAuthzTokenSource), nil
}

func readkeys(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	encoded, err := os.ReadFile(path)
	if err != nil {
		return "", errorsx.Wrapf(err, "unable to read signing keys: %s", path)
	}

	return string(encoded), nil
}

func visibility(n *debrepo.Namespace) string {
	if n.GetPublic() {
		return "public"
	}

	return "private"
}

// instructions for configuring apt to install from the namespace.
func instructions(n *debrepo.Namespace) string {
	host := debrepo.Hostname(n)
	keyring := fmt.Sprintf("/etc/apt/keyrings/%s.asc", n.GetName())
	s := fmt.Sprintf(`# install the namespace's signing keys
sudo tee %s > /dev/null <<'KEYS'
%s
KEYS

# add the repository, replace <distribution> with the distribution packages were published to
echo "deb [signed-by=%s] https://%s <distribution> main" | sudo tee /etc/apt/sources.list.d/%s.list
`, keyring, n.GetSigningKeys(), keyring, host, n.GetName())

	if !n.GetPublic() {
		s += fmt.Sprintf(`
# private namespaces require a token with deb read access, see eg deb token
printf 'machine %s\nlogin eg\npassword %%s\n' "$(eg deb token)" | sudo tee /etc/apt/auth.conf.d/%s.conf
sudo chmod 600 /etc/apt/auth.conf.d/%s.conf
`, host, n.GetName(), n.GetName())
	}

	return s
}

type Create struct {
	auth
	Name        string `arg:"" name:"name" help:"name of the namespace, must be a lowercase dns label"`
	Public      bool   `name:"public" help:"allow anyone to install packages from the namespace"`
	Description string `name:"description" help:"optional description"`
	Keys        string `name:"keys" help:"path to the armored gpg public keys that sign the repository, e.g. gpg --armor --export <key id>" type:"existingfile" required:""`
}

func (t Create) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	c, err := t.client(gctx, tlsc)
	if err != nil {
		return err
	}

	keys, err := readkeys(t.Keys)
	if err != nil {
		return err
	}

	n, err := debrepo.Create(gctx.Context, c, &debrepo.Namespace{
		Name:        t.Name,
		Public:      t.Public,
		Description: t.Description,
		SigningKeys: keys,
	})
	if err != nil {
		return err
	}

	fmt.Printf("created %s namespace %s served at https://%s\n", visibility(n), n.GetName(), debrepo.Hostname(n))
	return nil
}

type List struct {
	auth
}

func (t List) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	c, err := t.client(gctx, tlsc)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVISIBILITY\tGENERATION\tHOST\tDESCRIPTION")

	// follow next until the last page, which resets the offset to zero.
	for offset := uint64(0); ; {
		rsp, err := debrepo.Search(gctx.Context, c, offset)
		if err != nil {
			return err
		}

		for _, n := range rsp.GetItems() {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", n.GetName(), visibility(n), n.GetGeneration(), debrepo.Hostname(n), n.GetDescription())
		}

		if offset = rsp.GetNext().GetOffset(); offset == 0 {
			break
		}
	}

	return w.Flush()
}

type Show struct {
	auth
	Name string `arg:"" name:"name" help:"name of the namespace"`
}

func (t Show) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	c, err := t.client(gctx, tlsc)
	if err != nil {
		return err
	}

	n, err := debrepo.Find(gctx.Context, c, t.Name)
	if err != nil {
		return err
	}

	fmt.Printf("namespace %s (%s) generation %d\n%s\n\n%s", n.GetName(), visibility(n), n.GetGeneration(), n.GetDescription(), instructions(n))
	return nil
}

type Edit struct {
	auth
	Name        string  `arg:"" name:"name" help:"name of the namespace"`
	Public      *bool   `name:"public" help:"allow anyone to install packages from the namespace" negatable:""`
	Description *string `name:"description" help:"new description"`
	Keys        string  `name:"keys" help:"path to armored gpg public keys replacing the current signing keys" type:"existingfile"`
}

func (t Edit) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	c, err := t.client(gctx, tlsc)
	if err != nil {
		return err
	}

	current, err := debrepo.Find(gctx.Context, c, t.Name)
	if err != nil {
		return err
	}

	keys, err := readkeys(t.Keys)
	if err != nil {
		return err
	}

	updated := &debrepo.Namespace{
		Public:      current.GetPublic(),
		Description: current.GetDescription(),
		SigningKeys: keys,
	}

	if t.Public != nil {
		updated.Public = *t.Public
	}

	if t.Description != nil {
		updated.Description = *t.Description
	}

	n, err := debrepo.Update(gctx.Context, c, t.Name, updated)
	if err != nil {
		return err
	}

	fmt.Printf("updated %s namespace %s\n", visibility(n), n.GetName())
	return nil
}

type Remove struct {
	auth
	Name string `arg:"" name:"name" help:"name of the namespace"`
}

func (t Remove) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	c, err := t.client(gctx, tlsc)
	if err != nil {
		return err
	}

	if err = debrepo.Remove(gctx.Context, c, t.Name); err != nil {
		return err
	}

	fmt.Printf("removed %s\n", t.Name)
	return nil
}

type Token struct {
	auth
}

func (t Token) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	token, err := compute.NewAuthzTokenSource(tlsc.DefaultClient(), signer, authn.EndpointCompute(), gctx.AccountID).Token()
	if err != nil {
		return err
	}

	fmt.Println(token.AccessToken)
	return nil
}
