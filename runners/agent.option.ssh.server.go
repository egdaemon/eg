package runners

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
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

// sshrootfs mirrors the container paths of the files static to the ssh server configuration.
//
//go:embed agent.option.ssh.server.rootfs
var sshrootfs embed.FS

// AgentOptionSSHServer configures the container's sshd to accept eg's own
// managed SSH identity (the same key backing `eg ssh key`, actl authorize,
// compute upload, etc) for the egd workload user, and publishes port to the
// host bound to the container's sshd. keypath is the private key of that
// identity. port == 0 disables the feature.
func AgentOptionSSHServer(ctx context.Context, ws workspaces.Context, keypath string, port int) AgentOption {
	if port == 0 {
		return AgentOptionNoop
	}

	volumes, err := sshserverfiles(ctx, filepath.Join(ws.RuntimeDir, "ssh"), keypath)
	if err != nil {
		log.Println("unable to configure ssh access, disabling", err)
		return AgentOptionNoop
	}

	return AgentOptionCompose(
		AgentOptionVolumes(volumes...),
		AgentOptionCommandLine("--publish", fmt.Sprintf("%d:22", port)),
	)
}

// sshserverfiles materializes the ssh server files into sshdir and returns the
// read only mounts placing each of them at the same relative path within the container.
func sshserverfiles(ctx context.Context, sshdir string, keypath string) (volumes []string, err error) {
	if err = os.MkdirAll(filepath.Dir(keypath), 0700); err != nil {
		return nil, errorsx.Wrap(err, "unable to create directory for eg's ssh identity")
	}

	seed := envx.String(errorsx.Must(uuid.NewV4()).String(), eg.EnvEGSSHSeed, "EG_ENTROPY_SEED")
	signer, err := sshx.AutoCached(sshx.NewKeyGenSeeded(seed), keypath)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to load or generate eg's ssh identity")
	}

	if err = fsx.MkDirs(0700, sshdir); err != nil {
		return nil, errorsx.Wrap(err, "unable to create ssh runtime directory")
	}

	if err = fsx.CloneTree(ctx, sshdir, "agent.option.ssh.server.rootfs", sshrootfs); err != nil {
		return nil, errorsx.Wrap(err, "unable to clone ssh server files")
	}

	authorizedkeys := filepath.Join(sshdir, "etc", "ssh", "authorized_keys", eg.DefaultUsername)
	if err = os.MkdirAll(filepath.Dir(authorizedkeys), 0755); err != nil {
		return nil, errorsx.Wrap(err, "unable to create authorized_keys directory")
	}

	if err = os.WriteFile(authorizedkeys, ssh.MarshalAuthorizedKey(signer.PublicKey()), 0600); err != nil {
		return nil, errorsx.Wrap(err, "unable to write authorized_keys")
	}

	// mount files, not directories, so the image's own files in those directories remain visible.
	err = fs.WalkDir(os.DirFS(sshdir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		volumes = append(volumes, AgentMountReadOnly(filepath.Join(sshdir, path), filepath.Join("/", path)))
		return nil
	})

	return volumes, errorsx.Wrap(err, "unable to determine ssh server mounts")
}
