package cmdssh

import (
	"fmt"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/sshx"
	"golang.org/x/crypto/ssh"
)

type Cmd struct {
	Key GenKey `cmd:"" name:"key" help:"generate a deterministic ssh key from a seed"`
}

type GenKey struct {
	Seed string `name:"seed" help:"seed for generating a deterministic key, useful for ci/cd" default:"${vars_entropy_seed}"`
	Path string `name:"path" help:"path for the generated key (private key and .pub file)" default:"${vars_ssh_key_path}"`
}

// Run generates a deterministic SSH key from the given seed and writes it to
// the target path. The generated files are a private key (PEM format) and a
// public key (.pub file).
//
// The key is stable: running the command again with the same seed produces the
// same key, and if the file already exists it is loaded from disk unchanged.
func (t GenKey) Run(gctx *cmdopts.Global) error {
	kg := sshx.NewKeyGenSeeded(t.Seed)
	signer, err := sshx.AutoCached(kg, t.Path)
	if err != nil {
		return fmt.Errorf("ssh key generation failed: %w", err)
	}

	pubkey := signer.PublicKey()
	fmt.Printf("key written to %s (fingerprint: %s)\n", t.Path, ssh.FingerprintSHA256(pubkey))
	fmt.Printf("public key: %s", string(ssh.MarshalAuthorizedKey(pubkey)))
	return nil
}
