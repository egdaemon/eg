package cmdgpg

import (
	"fmt"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/gpgx"
)

type GpgCmd struct {
	Keyring CmdKeyring `cmd:"" name:"keyring" help:"generate a deterministic gpg keyring from a seed"`
}

type CmdKeyring struct {
	Seed      string `name:"seed" help:"seed for generating a deterministic keyring, useful for ci/cd" default:"${vars_entropy_seed}"`
	Directory string `name:"directory" short:"d" help:"gpg home directory to write the keyring into" default:"${vars_gpg_directory}"`
	Name      string `name:"name" help:"identity name to embed in the key" default:"${vars_user_name}"`
	Email     string `name:"email" help:"identity email to embed in the key" default:""`
}

func (t CmdKeyring) Run(gctx *cmdopts.Global) error {
	entity, err := gpgx.Keyring(t.Directory, t.Seed, gpgx.OptionKeyGenIdentity(t.Name, "", t.Email))
	if err != nil {
		return fmt.Errorf("gpg keyring generation failed: %w", err)
	}

	fmt.Printf("keyring written to %s (key id: %016X)\n", t.Directory, entity.PrimaryKey.KeyId)
	return nil
}
