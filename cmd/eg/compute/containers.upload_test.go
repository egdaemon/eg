package compute

import (
	"github.com/egdaemon/eg/internal/userx"
	"os"
	"testing"

	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

// every command that uploads a workload must satisfy what the enqueue endpoint enforces,
// see enqueueapi.
func TestContainersUpload(t *testing.T) {
	// the identities are remembered in the cache, keep it out of the user's.
	t.Setenv(userx.EnvCacheDirectory, t.TempDir())
	t.Setenv("RUNTIME_DIRECTORY", t.TempDir())

	tlsc, accepted := enqueueapi(t)
	_, sshkey := uploadrepo(t)
	require.NoError(t, os.WriteFile("Containerfile", []byte("FROM scratch\n"), 0600))

	cmd := c8sUpload{
		Containerfile: "Containerfile",
		SSHKeyPath:    sshkey,
		Endpoint:      "https://api.egdaemon.com/c/q/",
		GitRemote:     "origin",
		GitReference:  "HEAD",
		uploadoptions: uploadoptions{Follow: true},
	}

	require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "account"}, tlsc, cmdidentity.New(t.Context(), "account", "", cmdidentity.Login), nil))
	requirevcs(t, "container workload - Containerfile", "main.wasm", accepted())
}
