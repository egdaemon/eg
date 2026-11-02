package compute

import (
	"github.com/egdaemon/eg/internal/userx"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/stretchr/testify/require"
)

// every command that uploads a workload must satisfy what the enqueue endpoint enforces,
// see enqueueapi.
func TestUpload(t *testing.T) {
	// the identities are remembered in the cache, keep it out of the user's.
	t.Setenv(userx.EnvCacheDirectory, t.TempDir())
	t.Setenv("RUNTIME_DIRECTORY", t.TempDir())

	tlsc, accepted := enqueueapi(t)
	dir, sshkey := uploadrepo(t)

	moddir := eg.DefaultModuleDirectory(dir)
	require.NoError(t, os.MkdirAll(moddir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(moddir, "main.go"), []byte(`package main

import (
"context"
"log"

"github.com/egdaemon/eg/runtime/wasi/eg"
)

func main() {
if err := eg.Perform(context.Background()); err != nil {
	log.Fatalln(err)
}
}
`), 0600))
	require.NoError(t, compile.InitGolang(t.Context(), moddir, cmdopts.ModPath()))
	require.NoError(t, compile.InitGolangTidy(t.Context(), moddir))

	cmd := upload{
		SSHKeyPath:    sshkey,
		Dir:           dir,
		Endpoint:      "https://api.egdaemon.com/c/q/",
		GitRemote:     "origin",
		GitReference:  "HEAD",
		uploadoptions: uploadoptions{CacheDir: t.TempDir(), Follow: true},
	}

	require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "account"}, tlsc, cmdidentity.New(t.Context(), "account", "", cmdidentity.Login), nil))
	requirevcs(t, "Uploaded: test - HEAD", "main.wasm", accepted())
}
