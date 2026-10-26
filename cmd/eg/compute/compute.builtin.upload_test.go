package compute

import (
	"testing"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

// every command that uploads a workload must satisfy what the enqueue endpoint enforces,
// see enqueueapi.
func TestBuiltinUpload(t *testing.T) {
	tlsc, accepted := enqueueapi(t)
	dir, sshkey := uploadrepo(t)

	cmd := builtinUpload{
		SSHKeyPath:    sshkey,
		Dir:           dir,
		Name:          "containers/clear",
		Endpoint:      "https://api.egdaemon.com/c/q/",
		GitRemote:     "origin",
		GitReference:  "HEAD",
		uploadoptions: uploadoptions{Follow: true},
	}

	require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "account"}, tlsc, nil))
	requirevcs(t, "builtin workload - containers/clear", "containers/clear/main.wasm", accepted())
}
