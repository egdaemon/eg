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
	Directory string `name:"directory" short:"d" help:"directory to write private.asc and public.asc into" default:"${vars_gpg_directory}"`
	Name      string `name:"name" help:"identity name to embed in the key" default:"${vars_user_name}"`
	Email     string `name:"email" help:"identity email to embed in the key" default:""`
}

// Run generates a deterministic GPG keyring from the given seed and writes
// private.asc and public.asc into the target directory.
//
// The generated files are armored PGP keys. GPG does not load them
// automatically — you must import the private key before gpg commands can use
// it:
//
//	GNUPGHOME=<directory> gpg --import <directory>/private.asc
//
// The key is stable: running the command again with the same seed produces the
// same key ID, so a second call with an existing directory is a no-op (the
// existing files are loaded from disk unchanged).
func (t CmdKeyring) Run(gctx *cmdopts.Global) error {
	entity, err := gpgx.Keyring(t.Directory, t.Seed, gpgx.OptionKeyGenIdentity(t.Name, "", t.Email))
	if err != nil {
		return fmt.Errorf("gpg keyring generation failed: %w", err)
	}

	fmt.Printf("keyring written to %s (key id: %016X)\n", t.Directory, entity.PrimaryKey.KeyId)
	fmt.Printf("to use with gpg: GNUPGHOME=%s gpg --import %s/private.asc\n", t.Directory, t.Directory)
	return nil
}
