package main

import (
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/actl"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestCmdDaemon(t *testing.T) {
	t.Run("authorize-seed and daemon build the same identity from the same raw seed", func(t *testing.T) {
		// this exists because `eg actl authorize seed` and `eg actl bootstrap env
		// daemon` are independent sibling commands with no shared state between
		// them, and `eg daemon` is simply the consumer of whatever seed value
		// those two produce/register. The only thing keeping the identity
		// authorize-seed registers in sync with the identity daemon later
		// reconstructs is that both are handed the exact same injected Entropy
		// and KeyGenSeeded functions: authorize-seed derives once from its own
		// explicit seed via the injected Entropy before keygen'ing; bootstrap env
		// performs that identical injected derivation to mint EG_ENTROPY_SEED;
		// daemon consumes that already-derived value directly via the same
		// injected keygen, with no further derivation. This pins that invariant
		// end to end: given the same raw human seed, deriving it once (as
		// authorize-seed and bootstrap env each independently do, through the
		// same injected Entropy) and feeding daemon that derived value directly
		// must produce the same fingerprint authorize-seed itself produces. If
		// either command's seed handling drifts from this shape, this test
		// catches it without needing any network mocking, since both halves are
		// now pure functions of injected dependencies.
		rawSeed := uuid.Must(uuid.NewV7()).String()

		asigner, err := actl.AuthorizeSecret{Seed: rawSeed}.Signer(cmdopts.GenerateEntropy, sshx.NewKeyGenSeeded)
		require.NoError(t, err)

		// modeling what bootstrap env's Run body does: derive once from the same
		// raw seed via the same injected Entropy, and that's what ends up in
		// EG_ENTROPY_SEED for daemon to consume.
		derived, err := cmdopts.GenerateEntropy(rawSeed)
		require.NoError(t, err)

		dsigner, err := daemon{Seed: derived, SSHKeyPath: filepath.Join(t.TempDir(), "id")}.signer(sshx.NewKeyGenSeeded)
		require.NoError(t, err)

		require.Equal(t,
			ssh.FingerprintSHA256(asigner.PublicKey()),
			ssh.FingerprintSHA256(dsigner.PublicKey()),
		)
	})
}
