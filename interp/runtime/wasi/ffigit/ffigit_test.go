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

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigit"
)

// a wasm module that only exports a single page of memory.
var memorymodule = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic, version
	0x05, 0x03, 0x01, 0x00, 0x01, // memory section: 1 memory, min 1 page
	0x07, 0x0a, 0x01, 0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00, // export "memory"
}

func TestCloneV2(t *testing.T) {
	t.Run("clones and mirrors a repository without credentials", func(t *testing.T) {
		ctx := t.Context()
		cachedir := t.TempDir()

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
		errcode := ffigit.CloneV2(filepath.Join(eg.DefaultModuleDirectory(cachedir), "repos"), dst, runtimedir)(
			ctx, m, time.Now().Add(time.Minute).UnixMicro(),
			uriptr, uint32(len(uri)),
			remoteptr, uint32(len(remote)),
			treeishptr, uint32(len(commit)),
			0, 0, 8,
		)
		require.Zero(t, errcode)

		_, err = os.Stat(filepath.Join(dst, "main.go"))
		require.NoError(t, err, "committed file should be checked out")

		mirrors, err := filepath.Glob(filepath.Join(cachedir, ".eg", "repos", "*.git"))
		require.NoError(t, err)
		require.Len(t, mirrors, 1, "the repository should be mirrored in the cache")
	})
}

func TestCloneV3(t *testing.T) {
	// a repository with a single commit to clone.
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
	run("tag", "v1.0.0")
	run("tag", "-a", "-m", "annotated", "v1.0.1")

	// a branch which diverges from the tags.
	run("checkout", "-q", "-b", "feature/branch")
	require.NoError(t, os.WriteFile(filepath.Join(src, "feature.go"), []byte("package main"), 0644))
	run("add", ".")
	run("commit", "-q", "-m", "feature")
	branchcommit := run("rev-parse", "HEAD")
	run("checkout", "-q", "-")

	uri, remote := "file://"+src, "origin"

	call := func(t *testing.T, cachedir string, remap func(string) string, dst string, treeish string) (errcode uint32) {
		ctx := t.Context()
		rt := wazero.NewRuntime(ctx)
		defer rt.Close(ctx)
		m, err := rt.InstantiateWithConfig(ctx, memorymodule, wazero.NewModuleConfig())
		require.NoError(t, err)

		dstptr, uriptr, remoteptr, treeishptr := uint32(0), uint32(1024), uint32(2048), uint32(3072)
		require.True(t, m.Memory().WriteString(dstptr, dst))
		require.True(t, m.Memory().WriteString(uriptr, uri))
		require.True(t, m.Memory().WriteString(remoteptr, remote))
		require.True(t, m.Memory().WriteString(treeishptr, treeish))

		return ffigit.CloneV3(cachedir, t.TempDir(), remap)(
			ctx, m, time.Now().Add(time.Minute).UnixMicro(),
			dstptr, uint32(len(dst)),
			uriptr, uint32(len(uri)),
			remoteptr, uint32(len(remote)),
			treeishptr, uint32(len(treeish)),
			0, 0, 8,
		)
	}

	t.Run("destinations are remapped to the host", func(t *testing.T) {
		host := t.TempDir()
		remap := func(s string) string { return filepath.Join(host, s) }

		require.Zero(t, call(t, t.TempDir(), remap, "/guest/cache/repo", commit))

		_, err := os.Stat(filepath.Join(host, "guest", "cache", "repo", "main.go"))
		require.NoError(t, err, "the clone should be at the remapped host path")
		_, err = os.Stat(filepath.Join("/guest", "cache", "repo"))
		require.ErrorIs(t, err, os.ErrNotExist, "the guest path should not be used on the host")
	})

	t.Run("blank destinations are rejected", func(t *testing.T) {
		host := t.TempDir()
		remap := func(s string) string { return filepath.Join(host, s) }

		require.NotZero(t, call(t, t.TempDir(), remap, "", commit))

		entries, err := os.ReadDir(host)
		require.NoError(t, err)
		require.Empty(t, entries, "nothing should be cloned")
	})

	t.Run("treeish", func(t *testing.T) {
		for _, cached := range []bool{false, true} {
			cached := cached
			name := "without a cache"
			if cached {
				name = "with a cache"
			}

			t.Run(name, func(t *testing.T) {
				host := t.TempDir()
				remap := func(s string) string { return filepath.Join(host, s) }
				cachedir := ""
				if cached {
					cachedir = t.TempDir()
				}

				tests := []struct {
					name    string
					treeish string
					head    string
					file    string
					missing string
				}{
					{name: "commit", treeish: commit, head: commit, file: "main.go", missing: "feature.go"},
					{name: "branch", treeish: "feature/branch", head: branchcommit, file: "feature.go"},
					{name: "lightweight tag", treeish: "v1.0.0", head: commit, file: "main.go", missing: "feature.go"},
					{name: "annotated tag", treeish: "v1.0.1", head: commit, file: "main.go", missing: "feature.go"},
				}

				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						dst := "/guest/" + strings.ReplaceAll(tc.name, " ", "-")
						require.Zero(t, call(t, cachedir, remap, dst, tc.treeish))

						out, err := exec.Command("git", "-C", remap(dst), "rev-parse", "HEAD").CombinedOutput()
						require.NoErrorf(t, err, "%s", out)
						require.Equal(t, tc.head, strings.TrimSpace(string(out)))

						_, err = os.Stat(filepath.Join(remap(dst), tc.file))
						require.NoError(t, err)

						if tc.missing != "" {
							_, err = os.Stat(filepath.Join(remap(dst), tc.missing))
							require.ErrorIs(t, err, os.ErrNotExist)
						}
					})
				}

				t.Run("unknown treeish", func(t *testing.T) {
					require.NotZero(t, call(t, cachedir, remap, "/guest/unknown", "does-not-exist"))
				})
			})
		}
	})
}
