package cmdapt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/apt"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
)

type Create struct {
	target   `embed:""`
	settings `embed:""`
}

func (t Create) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) error {
	c, err := t.meta(gctx, tlsc, ident)
	if err != nil {
		return err
	}

	out, err := c.Create(gctx.Context, t.settings.settings())
	if err != nil {
		return explain(err, "create repository")
	}

	return printjson(os.Stdout, out)
}

type Show struct {
	target `embed:""`
}

func (t Show) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) error {
	c, err := t.ci(gctx, tlsc, ident)
	if err != nil {
		return err
	}

	out, err := c.Show(gctx.Context)
	if err != nil {
		return explain(err, "show repository")
	}

	return printjson(os.Stdout, out)
}

type Set struct {
	target           `embed:""`
	settings         `embed:""`
	KeepN            *int           `name:"keepn" help:"number of published versions to keep"`
	KeepNGrace       *time.Duration `name:"keepn-grace" help:"how long versions beyond keepn are kept, for example 2160h"`
	UnpublishedGrace *time.Duration `name:"unpublished-grace" help:"how long uploaded but unpublished packages are kept, for example 2160h"`
}

func (t Set) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) error {
	s := t.settings.settings()
	s.KeepN = t.KeepN
	s.KeepNGracePeriod = duration(t.KeepNGrace)
	s.UnpublishedGracePeriod = duration(t.UnpublishedGrace)

	c, err := t.meta(gctx, tlsc, ident)
	if err != nil {
		return err
	}

	out, err := c.Set(gctx.Context, s)
	if err != nil {
		return explain(err, "change repository")
	}

	return printjson(os.Stdout, out)
}

type Key struct {
	target `embed:""`
	Output string `name:"output" short:"o" help:"file the key is written to, - for stdout; defaults to /etc/apt/keyrings/<repository>.asc"`
}

func (t Key) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	c, err := t.anonymous(gctx, tlsc, ident)
	if err != nil {
		return err
	}

	output := t.Output
	if output == "" {
		output = keyring(t.Name)
	}

	if output == "-" {
		return explain(c.Key(gctx.Context, os.Stdout), "fetch key")
	}

	return writekey(output, func(dst io.Writer) error {
		return explain(c.Key(gctx.Context, dst), "fetch key")
	})
}

// writekey writes the key to a temporary file in the same directory and renames it into place, a
// failed download never replaces an existing key.
func writekey(path string, fetch func(io.Writer) error) (err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if err = fetch(tmp); err != nil {
		return err
	}

	if err = tmp.Chmod(0644); err != nil {
		return err
	}

	if err = tmp.Close(); err != nil {
		return err
	}

	return errorsx.Wrap(os.Rename(tmp.Name(), path), "unable to install key")
}

func keyring(repo string) string {
	return filepath.Join("/etc/apt/keyrings", repo+".asc")
}

type Sources struct {
	target    `embed:""`
	Suite     string `name:"suite" help:"suite of the sources line" default:"stable"`
	Component string `name:"component" help:"component of the sources line" default:"main"`
	Basic     bool   `name:"basic" help:"use https with basic authentication instead of the egs transport, for machines without eg installed"`
}

func (t Sources) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) error {
	if err := apt.ValidName(t.Name); err != nil {
		return err
	}

	signer, err := t.Signer()
	if err != nil {
		return err
	}

	aid, err := t.Account(gctx, tlsc, ident, signer)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(os.Stdout, sources(t.AptDomain, aid, t.Name, t.Suite, t.Component, t.Basic))
	return err
}

// sources renders the instructions for consuming the repository.
func sources(domain, aid, repo, suite, component string, basic bool) string {
	var (
		key    = keyring(repo)
		list   = filepath.Join("/etc/apt/sources.list.d", repo+".list")
		scheme = apt.Scheme
	)

	if basic {
		scheme = "https"
	}

	line := fmt.Sprintf("deb [signed-by=%s] %s %s %s", key, apt.URL(scheme, domain, aid, repo), suite, component)

	if !basic {
		return fmt.Sprintf(`# install the signing key
sudo eg apt repo key %s --account-id=%s --output=%s

# add the repository, the egs transport authorizes requests with the eg identity of the machine
echo '%s' | sudo tee %s
`, repo, aid, key, line, list)
	}

	auth := filepath.Join("/etc/apt/auth.conf.d", repo+".conf")
	return fmt.Sprintf(`# install the signing key
curl -fsSL %s/key.asc | sudo tee %s >/dev/null

# add the repository
echo '%s' | sudo tee %s

# credentials, tokens expire: refresh this file with a new token before it does
printf 'machine %s/%s\nlogin token\npassword %%s\n' "$(eg authz ci token --account-id=%s)" | sudo sh -c 'umask 077 && cat > %s'
`, apt.URL("https", domain, aid, repo), key, line, list, apt.Host(domain, aid), repo, aid, auth)
}
