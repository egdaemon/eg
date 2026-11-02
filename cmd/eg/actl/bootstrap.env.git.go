package actl

import (
	"context"
	"log"
	"os/exec"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
)

type BootstrapEnvGit struct {
	cmdopts.VCS `embed:""`
	Dir         string `name:"directory" help:"root directory of the repository" default:"${vars_git_directory}"`
	Global      bool   `name:"global" help:"write the global git config instead of the repository's"`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t BootstrapEnvGit) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	c := tlsc.DefaultClient()

	db, err := ident.Store()
	if err != nil {
		return err
	}
	defer db.Close()

	r, err := identity.QueryVCSAuto(gctx.AccountID, gctx.ProfileID, t.VcsURI).Resolve(gctx.Context, db, c, signer, ident.Choose)
	if err != nil {
		return err
	}

	// a remembered identity only carries ids, the name and email come from the profile.
	authed, err := identity.Profiles(gctx.Context, c, signer, gctx.AccountID, gctx.ProfileID)
	if err != nil {
		return err
	}

	name, email, err := gituser(r.Current, authed.Profiles...)
	if err != nil {
		return err
	}

	if err = writegitconfig(gctx.Context, t.Dir, t.Global, name, email); err != nil {
		return err
	}

	log.Println("git user", name, email)

	return nil
}

// gituser the name and email git commits with for the resolved profile, found within the profiles of the ssh key.
func gituser(current *authn.Authn, profiles ...*authn.Authn) (name, email string, err error) {
	p, found := slicesx.Find(func(v *authn.Authn) bool {
		return stringsx.Present(current.GetProfile().GetId()) && v.GetProfile().GetId() == current.GetProfile().GetId()
	}, profiles...)

	if !found {
		p, found = slicesx.Find(func(v *authn.Authn) bool {
			return v.GetProfile().GetAccountId() == stringsx.FirstNonBlank(current.GetAccount().GetId(), current.GetProfile().GetAccountId())
		}, profiles...)
	}

	if !found {
		return "", "", errorsx.UserFriendly(errorsx.String("unable to find the resolved profile"))
	}

	name, email = p.GetProfile().GetDisplay(), p.GetProfile().GetEmail()
	if stringsx.Blank(name) {
		return "", "", errorsx.UserFriendly(errorsx.String("the profile has no display name to use as the git user name"))
	}

	if stringsx.Blank(email) {
		return "", "", errorsx.UserFriendly(errorsx.String("the profile has no email to use as the git user email"))
	}

	return name, email, nil
}

// writegitconfig sets user.name and user.email within the git config of the repository at dir, or the global config.
func writegitconfig(ctx context.Context, dir string, global bool, name, email string) (err error) {
	scope := "--local"
	if global {
		scope = "--global"
	} else if !gitx.IsRepository(dir) {
		return errorsx.UserFriendly(errorsx.Errorf("%s is not a git repository, use --global to write the global git config", dir))
	}

	for k, v := range map[string]string{"user.name": name, "user.email": email} {
		cmd := exec.CommandContext(ctx, "git", "-C", dir, "config", scope, k, v)
		if out, err := cmd.CombinedOutput(); err != nil {
			return errorsx.Wrapf(err, "unable to set %s: %s", k, string(out))
		}
	}

	return nil
}
