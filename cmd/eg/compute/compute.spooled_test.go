package compute

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

func TestSpooledArgs(t *testing.T) {
	t.Run("archive defaults to stdin", func(t *testing.T) {
		var cli struct {
			Spooled spooled `cmd:""`
		}
		k, err := kong.New(&cli)
		require.NoError(t, err)
		_, err = k.Parse([]string{"spooled"})
		require.NoError(t, err)
		require.Equal(t, "-", cli.Spooled.Archive)
		require.Empty(t, cli.Spooled.Spool)
	})

	t.Run("accepts an archive path and spool directory", func(t *testing.T) {
		var cli struct {
			Spooled spooled `cmd:""`
		}
		k, err := kong.New(&cli)
		require.NoError(t, err)
		_, err = k.Parse([]string{"spooled", "--spool", "/tmp/spool", "workload.tar.gz"})
		require.NoError(t, err)
		require.Equal(t, "workload.tar.gz", cli.Spooled.Archive)
		require.Equal(t, "/tmp/spool", cli.Spooled.Spool)
	})
}

func TestSpooledEntry(t *testing.T) {
	t.Run("detects a nested module", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "containers", Typeflag: tar.TypeDir, Mode: 0755}))
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "containers/main.wasm", Size: 1, Mode: 0644}))
		_, err := tw.Write([]byte("x"))
		require.NoError(t, err)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: ".eg.env", Size: 1, Mode: 0644}))
		_, err = tw.Write([]byte("x"))
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())

		entry, err := spooledentry(&buf)
		require.NoError(t, err)
		require.Equal(t, "containers/main.wasm", entry)
	})

	t.Run("requires exactly one module", func(t *testing.T) {
		for _, names := range [][]string{{}, {"a/main.wasm", "b/main.wasm"}} {
			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gw)
			for _, name := range names {
				require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Size: 1, Mode: 0644}))
				_, err := tw.Write([]byte("x"))
				require.NoError(t, err)
			}
			require.NoError(t, tw.Close())
			require.NoError(t, gw.Close())

			_, err := spooledentry(&buf)
			require.Error(t, err)
			require.Contains(t, err.Error(), "--entry")
		}
	})

	t.Run("rejects a non archive", func(t *testing.T) {
		_, err := spooledentry(bytes.NewReader([]byte("not an archive")))
		require.Error(t, err)
	})
}

// end to end tests, these push an archive through the real spool and runners.QueueN
// and therefore require podman.
func TestSpooledRun(t *testing.T) {
	module, err := os.ReadFile(filepath.Join("..", "..", "..", "runners", ".fixtures", "example.1.wasm"))
	require.NoError(t, err)

	t.Run("runs an archive file through the queue", func(t *testing.T) {
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
		require.NoError(t, spooled{Archive: archive, Spool: spool}.Run(&cmdopts.Global{Context: t.Context()}, new(cmdopts.HotswapPath)))

		queued, err := os.ReadDir(filepath.Join(spool, "q"))
		require.NoError(t, err)
		require.Empty(t, queued, "the workload was consumed from the queue")
	})

	t.Run("runs an archive from stdin through the queue", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "mod", Typeflag: tar.TypeDir, Mode: 0755}))
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "mod/main.wasm", Size: int64(len(module)), Mode: 0644}))
		_, err := tw.Write(module)
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())

		stdin := filepath.Join(t.TempDir(), "stdin")
		require.NoError(t, os.WriteFile(stdin, buf.Bytes(), 0600))
		in, err := os.Open(stdin)
		require.NoError(t, err)
		defer in.Close()

		orig := os.Stdin
		os.Stdin = in
		defer func() { os.Stdin = orig }()

		require.NoError(t, spooled{Archive: "-", Spool: t.TempDir()}.Run(&cmdopts.Global{Context: t.Context()}, new(cmdopts.HotswapPath)))
	})

	t.Run("a failing workload fails the command", func(t *testing.T) {
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
		require.Error(t, spooled{Archive: archive, Spool: t.TempDir()}.Run(&cmdopts.Global{Context: t.Context()}, new(cmdopts.HotswapPath)))
	})

	t.Run("a missing archive fails the command", func(t *testing.T) {
		require.Error(t, spooled{Archive: filepath.Join(t.TempDir(), "missing.tar.gz"), Spool: t.TempDir()}.Run(&cmdopts.Global{Context: t.Context()}, new(cmdopts.HotswapPath)))
	})
}
