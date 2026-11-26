package podmanx

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
)

// workloadconf is the podman configuration of the containers eg runs within a workload container.
//
//go:embed workload.conf
var workloadconf []byte

const envconfoverride = "CONTAINERS_CONF_OVERRIDE"

// Workload reports whether this eg runs within a workload container, as the eg that launched the
// container declared (eg.EnvComputePodmanWorkload). nothing is detected: a host is never declared one.
func Workload() bool {
	return envx.Boolean(false, eg.EnvComputePodmanWorkload)
}

var workloadconfpath = sync.OnceValues(func() (string, error) {
	return writeworkloadconf(os.TempDir())
})

// writeworkloadconf writes workloadconf into dir, named by its content: every eg of the same
// configuration shares the file and none rewrites it in place while another's podman reads it.
func writeworkloadconf(dir string) (path string, err error) {
	digest := sha256.Sum256(workloadconf)
	path = filepath.Join(dir, "eg.podman."+hex.EncodeToString(digest[:8])+".conf")

	if _, err = os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", errorsx.Wrap(err, "unable to stat the podman configuration")
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return "", errorsx.Wrap(err, "unable to write the podman configuration")
	}
	defer os.Remove(tmp.Name())

	if _, err = tmp.Write(workloadconf); err != nil {
		return "", errorsx.Compact(err, tmp.Close())
	}

	if err = tmp.Chmod(0644); err != nil {
		return "", errorsx.Compact(err, tmp.Close())
	}

	if err = tmp.Close(); err != nil {
		return "", err
	}

	return path, errorsx.Wrap(os.Rename(tmp.Name(), path), "unable to write the podman configuration")
}

// Environ is environ for a podman command eg runs: when Workload it points podman at workloadconf,
// only the podman commands see it, never eg itself nor what a workload runs. an override environ
// already has is the user's, it wins.
func Environ(environ []string) []string {
	return environ_(environ, Workload(), workloadconfpath)
}

func environ_(environ []string, workload bool, conf func() (string, error)) []string {
	if !workload {
		return environ
	}

	for _, kv := range environ {
		if strings.HasPrefix(kv, envconfoverride+"=") {
			return environ
		}
	}

	path, err := conf()
	if err != nil {
		// podman runs with its own configuration, the failure it reports names the cause.
		log.Println("podman workload configuration skipped", err)
		return environ
	}

	return append(append(make([]string, 0, len(environ)+1), environ...), envconfoverride+"="+path)
}

// Command is a podman command, see Environ.
func Command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Env = Environ(os.Environ())
	return cmd
}
