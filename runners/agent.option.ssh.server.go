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
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/ssh"
)

// sshAuthorizedKeysUser is the workload user sshd will authenticate for; the
// container has no other interactive login user.
const sshAuthorizedKeysUser = eg.DefaultUsername

// AgentOptionSSHServer configures the container's sshd to accept eg's own
// managed SSH identity (the same key backing `eg ssh key`, actl authorize,
// compute upload, etc) for the egd workload user, and publishes port to the
// host bound to the container's sshd. port == 0 disables the feature.
func AgentOptionSSHServer(ctx context.Context, ws workspaces.Context, port int) AgentOption {
	if port == 0 {
		return AgentOptionNoop
	}

	keypath := userx.DefaultSSHKeyPath()
	if err := os.MkdirAll(filepath.Dir(keypath), 0700); err != nil {
		log.Println("unable to create directory for eg's ssh identity, disabling ssh access", err)
		return AgentOptionNoop
	}

	seed := envx.String(errorsx.Must(uuid.NewV4()).String(), eg.EnvEGSSHSeed, "EG_ENTROPY_SEED")
	signer, err := sshx.AutoCached(sshx.NewKeyGenSeeded(seed), keypath)
	if err != nil {
		log.Println("unable to load or generate eg's ssh identity, disabling ssh access", err)
		return AgentOptionNoop
	}

	sshdir := filepath.Join(ws.RuntimeDir, "ssh")
	if err := os.MkdirAll(sshdir, 0700); err != nil {
		log.Println("unable to create ssh runtime directory, disabling ssh access", err)
		return AgentOptionNoop
	}

	authorizedkeys := filepath.Join(sshdir, "authorized_keys")
	if err := os.WriteFile(authorizedkeys, ssh.MarshalAuthorizedKey(signer.PublicKey()), 0644); err != nil {
		log.Println("unable to write authorized_keys, disabling ssh access", err)
		return AgentOptionNoop
	}

	dropindir := filepath.Join(sshdir, "sshd_config.d")
	if err := os.MkdirAll(dropindir, 0700); err != nil {
		log.Println("unable to create sshd_config.d directory, disabling ssh access", err)
		return AgentOptionNoop
	}

	dropin := filepath.Join(dropindir, "99-eg.conf")
	dropincontent := "AuthorizedKeysFile /etc/ssh/authorized_keys/%u .ssh/authorized_keys\n"
	if err := os.WriteFile(dropin, []byte(dropincontent), 0644); err != nil {
		log.Println("unable to write sshd_config.d drop-in, disabling ssh access", err)
		return AgentOptionNoop
	}

	return AgentOptionCompose(
		AgentOptionVolumes(
			AgentMountReadOnly(authorizedkeys, filepath.Join("/etc/ssh/authorized_keys", sshAuthorizedKeysUser)),
			AgentMountReadOnly(dropin, "/etc/ssh/sshd_config.d/99-eg.conf"),
		),
		AgentOptionCommandLine("--publish", fmt.Sprintf("%d:22", port)),
	)
}
