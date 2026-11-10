package ffigit_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffigit"
)

// a wasm module that only exports a single page of memory.
var memorymodule = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic, version
	0x05, 0x03, 0x01, 0x00, 0x01, // memory section: 1 memory, min 1 page
	0x07, 0x0a, 0x01, 0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00, // export "memory"
}

func TestCloneV2WithoutCredentials(t *testing.T) {
	ctx := t.Context()

	src := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", src}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(src, "main.go"), []byte("package main"), 0644))
	run("add", ".")
	run("commit", "-q", "-m", "init")
	commit := run("rev-parse", "HEAD")

	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	m, err := rt.InstantiateWithConfig(ctx, memorymodule, wazero.NewModuleConfig())
	require.NoError(t, err)

	uri, remote := "file://"+src, "origin"
	uriptr, remoteptr, treeishptr := uint32(0), uint32(1024), uint32(2048)
	require.True(t, m.Memory().WriteString(uriptr, uri))
	require.True(t, m.Memory().WriteString(remoteptr, remote))
	require.True(t, m.Memory().WriteString(treeishptr, commit))

	dst := filepath.Join(t.TempDir(), "clone")
	runtimedir := t.TempDir() // no vcsaccess.token, so no stored credentials.

	// no credentials stored, and no environment variables.
	errcode := ffigit.CloneV2(dst, runtimedir)(
		ctx, m, time.Now().Add(time.Minute).UnixMicro(),
		uriptr, uint32(len(uri)),
		remoteptr, uint32(len(remote)),
		treeishptr, uint32(len(commit)),
		0, 0, 8,
	)
	require.Zero(t, errcode)

	_, err = os.Stat(filepath.Join(dst, "main.go"))
	require.NoError(t, err, "committed file should be checked out")
}
