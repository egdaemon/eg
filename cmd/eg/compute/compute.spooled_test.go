package compute

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/runners"
	"github.com/stretchr/testify/require"
)

func TestSpooledArgs(t *testing.T) {
	t.Run("archive defaults to stdin", func(t *testing.T) {
		var cli struct {
			Spooled spooled `cmd:""`
		}
		k, err := kong.New(&cli, cmdopts.RuntimeResources{Arch: "amd64", OS: "linux"}.KongVars())
		require.NoError(t, err)
		_, err = k.Parse([]string{"spooled"})
		require.NoError(t, err)
		archive, err := cli.Spooled.Archive.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "-", string(archive))
		require.Empty(t, cli.Spooled.Spool)
	})

	t.Run("accepts an archive path and spool directory", func(t *testing.T) {
		var cli struct {
			Spooled spooled `cmd:""`
		}
		k, err := kong.New(&cli, cmdopts.RuntimeResources{Arch: "amd64", OS: "linux"}.KongVars())
		require.NoError(t, err)
		_, err = k.Parse([]string{"spooled", "--spool", "/tmp/spool", "workload.tar.gz"})
		require.NoError(t, err)
		archive, err := cli.Spooled.Archive.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "workload.tar.gz", string(archive))
		require.Equal(t, "/tmp/spool", cli.Spooled.Spool)
	})
}

func TestSpooledEntry(t *testing.T) {
	// the archive of a workspace holds everything built in it, i.e. the modules of earlier builds, and the modules
	// generated for a module within its .wasm.d directory.
	multi := []string{
		"tests/concurrent/main.wasm",
		"tests/concurrent/main.wasm.d/module.21.3.wasm",
		"tests/stress/main.wasm",
		"tests/stress/main.wasm.d/module.7.1.wasm",
		"tests/stress2/main.wasm",
		"tests/envvars/main.wasm",
		"environ.env",
	}

	for _, tc := range []struct {
		name     string
		files    []string
		prefix   string
		expected string
		failed   bool
	}{
		{name: "detects a single module", files: []string{"containers/main.wasm", ".eg.env"}, expected: "containers/main.wasm"},
		{name: "ignores generated modules when detecting", files: []string{"mod/main.wasm", "mod/main.wasm.d/module.1.wasm"}, expected: "mod/main.wasm"},
		{name: "selects a module by prefix", files: multi, prefix: "tests/stress", expected: "tests/stress/main.wasm"},
		{name: "prefix tolerates a trailing slash", files: multi, prefix: "tests/stress/", expected: "tests/stress/main.wasm"},
		{name: "prefix matches whole path segments", files: multi, prefix: "tests/stress2", expected: "tests/stress2/main.wasm"},
		{name: "prefix can be the full path", files: multi, prefix: "tests/envvars/main.wasm", expected: "tests/envvars/main.wasm"},
		{name: "a prefix covering several modules is ambiguous", files: multi, prefix: "tests", failed: true},
		{name: "a partial segment does not match", files: multi, prefix: "tests/str", failed: true},
		{name: "a prefix matching nothing fails", files: multi, prefix: "tests/missing", failed: true},
		{name: "several modules need a prefix", files: multi, failed: true},
		{name: "no module fails", files: []string{".eg.env"}, failed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gw)
			for _, name := range tc.files {
				require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Size: 1, Mode: 0644}))
				_, err := tw.Write([]byte("x"))
				require.NoError(t, err)
			}
			require.NoError(t, tw.Close())
			require.NoError(t, gw.Close())

			entry, err := spooledentry(&buf, tc.prefix)
			if tc.failed {
				require.Error(t, err)
				require.Contains(t, err.Error(), "--entry")
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expected, entry)
		})
	}

	t.Run("rejects a non archive", func(t *testing.T) {
		_, err := spooledentry(bytes.NewReader([]byte("not an archive")), "")
		require.Error(t, err)
	})
}

// end to end tests, these push an archive through the real spool and runners.QueueN
// and therefore require podman.
func TestSpooledRun(t *testing.T) {
	// the tests run nested as an unprivileged user in CI, where podman cannot create the containers: no systemd scopes
	// and no fresh proc mounts. the same path is exercised, as root, by the archive | spooled integration test in .eg/main.go.
	if runners.Nested() {
		t.Skip("podman in podman as an unprivileged user is not supported, see the spooled integration test")
	}

	module, err := os.ReadFile(filepath.Join("..", "..", "..", "runners", ".fixtures", "example.1.wasm"))
	require.NoError(t, err)

	t.Run("runs an archive file through the queue", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		archive := filepath.Join(t.TempDir(), "workload.tar.gz")
		dst, err := os.Create(archive)
		require.NoError(t, err)
		gw := gzip.NewWriter(dst)
		tw := tar.NewWriter(gw)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "mod", Typeflag: tar.TypeDir, Mode: 0755}))
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "mod/main.wasm", Size: int64(len(module)), Mode: 0644}))
		_, err = tw.Write(module)
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())
		require.NoError(t, dst.Close())

		spool := t.TempDir()
		var input cmdopts.IOIn
		require.NoError(t, input.UnmarshalText([]byte(archive)))
		require.NoError(t, spooled{Archive: input, Spool: spool}.Run(&cmdopts.Global{Context: ctx}, new(cmdopts.HotswapPath), bytes.NewReader(nil)))

		queued, err := os.ReadDir(filepath.Join(spool, "q"))
		require.NoError(t, err)
		require.Empty(t, queued, "the workload was consumed from the queue")
	})

	t.Run("runs an archive from stdin through the queue", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "mod", Typeflag: tar.TypeDir, Mode: 0755}))
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "mod/main.wasm", Size: int64(len(module)), Mode: 0644}))
		_, err := tw.Write(module)
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())

		var input cmdopts.IOIn
		require.NoError(t, input.UnmarshalText([]byte("-")))
		require.NoError(t, spooled{Archive: input, Spool: t.TempDir()}.Run(&cmdopts.Global{Context: ctx}, new(cmdopts.HotswapPath), &buf))
	})

	t.Run("a failing workload fails the command", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "mod", Typeflag: tar.TypeDir, Mode: 0755}))
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "mod/main.wasm", Size: 12, Mode: 0644}))
		_, err := tw.Write([]byte("not a module"))
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())

		archive := filepath.Join(t.TempDir(), "workload.tar.gz")
		require.NoError(t, os.WriteFile(archive, buf.Bytes(), 0600))
		var input cmdopts.IOIn
		require.NoError(t, input.UnmarshalText([]byte(archive)))
		require.Error(t, spooled{Archive: input, Spool: t.TempDir()}.Run(&cmdopts.Global{Context: ctx}, new(cmdopts.HotswapPath), bytes.NewReader(nil)))
	})

	t.Run("a missing archive fails the command", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		var input cmdopts.IOIn
		require.NoError(t, input.UnmarshalText([]byte(filepath.Join(t.TempDir(), "missing.tar.gz"))))
		require.Error(t, spooled{Archive: input, Spool: t.TempDir()}.Run(&cmdopts.Global{Context: ctx}, new(cmdopts.HotswapPath), bytes.NewReader(nil)))
	})
}
