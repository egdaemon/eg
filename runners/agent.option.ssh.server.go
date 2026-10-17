package runners

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/ssh"
)

// AgentOptionSSHServer configures the container's sshd to accept eg's own
// managed SSH identity (the same key backing `eg ssh key`, actl authorize,
// compute upload, etc) for the egd workload user, and publishes port to the
// host bound to the container's sshd. keypath is the private key of that
// identity. port == 0 disables the feature.
//
// the sshd configuration (sshd_config.d drop-in, profile.d) ships in the
// egworkload package, only the authorized key is specific to the invocation.
func AgentOptionSSHServer(ctx context.Context, ws workspaces.Context, keypath string, port int) AgentOption {
	if port == 0 {
		return AgentOptionNoop
	}

	authorizedkeys, err := sshauthorizedkeys(filepath.Join(ws.RuntimeDir, "ssh"), keypath)
	if err != nil {
		log.Println("unable to configure ssh access, disabling", err)
		return AgentOptionNoop
	}

	return AgentOptionCompose(
		AgentOptionVolumes(
			AgentMountReadOnly(authorizedkeys, filepath.Join("/etc/ssh/authorized_keys", eg.DefaultUsername)),
		),
		AgentOptionCommandLine("--publish", fmt.Sprintf("%d:22", port)),
	)
}

// sshauthorizedkeys writes the public key of eg's ssh identity into sshdir and returns its path.
func sshauthorizedkeys(sshdir string, keypath string) (path string, err error) {
	if err = os.MkdirAll(filepath.Dir(keypath), 0700); err != nil {
		return "", errorsx.Wrap(err, "unable to create directory for eg's ssh identity")
	}

	seed := envx.String(errorsx.Must(uuid.NewV4()).String(), eg.EnvEGSSHSeed, "EG_ENTROPY_SEED")
	signer, err := sshx.AutoCached(sshx.NewKeyGenSeeded(seed), keypath)
	if err != nil {
		return "", errorsx.Wrap(err, "unable to load or generate eg's ssh identity")
	}

	if err = fsx.MkDirs(0700, sshdir); err != nil {
		return "", errorsx.Wrap(err, "unable to create ssh runtime directory")
	}

	// sshd reads the file as egd, not as the owner of the mounted file.
	path = filepath.Join(sshdir, "authorized_keys")
	if err = os.WriteFile(path, ssh.MarshalAuthorizedKey(signer.PublicKey()), 0644); err != nil {
		return "", errorsx.Wrap(err, "unable to write authorized_keys")
	}

	return path, nil
}
