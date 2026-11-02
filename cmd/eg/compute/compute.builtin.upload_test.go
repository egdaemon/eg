package compute

import (
	"github.com/egdaemon/eg/internal/userx"
	"testing"

	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

// every command that uploads a workload must satisfy what the enqueue endpoint enforces,
// see enqueueapi.
func TestBuiltinUpload(t *testing.T) {
	// the identities are remembered in the cache, keep it out of the user's.
	t.Setenv(userx.EnvCacheDirectory, t.TempDir())
	t.Setenv("RUNTIME_DIRECTORY", t.TempDir())

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

	require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "account"}, tlsc, cmdidentity.New(t.Context(), "account", "", cmdidentity.Login), nil))
	requirevcs(t, "builtin workload - containers/clear", "containers/clear/main.wasm", accepted())
}
