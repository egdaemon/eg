package compute

import (
	"os"
	"testing"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

// every command that uploads a workload must satisfy what the enqueue endpoint enforces,
// see enqueueapi.
func TestContainersUpload(t *testing.T) {
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

	require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "account"}, tlsc, nil))
	requirevcs(t, "container workload - Containerfile", "main.wasm", accepted())
}
