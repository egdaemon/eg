package interp

import (
	"archive/tar"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCell(t *testing.T) {
	var (
		buf bytes.Buffer
		out bytes.Buffer
	)

	wasm := interptest.Build(t, "./interp/.xfixtures/cell")

	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "etc/", Mode: 0755}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644, Size: 4}))
	_, err := tw.Write([]byte("egd\n"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	toc, err := egiso.Build(io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len())), &out)
	require.NoError(t, err)
	image := filepath.Join(t.TempDir(), "root.egiso")
	require.NoError(t, os.WriteFile(image, out.Bytes(), 0600))

	svc, err := grpc.NewClient("passthrough:///unused", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { svc.Close() })

	workspace := func(t *testing.T) workspaces.Context {
		return workspaces.Context{
			Root:           t.TempDir(),
			RuntimeDir:     t.TempDir(),
			CacheDir:       t.TempDir(),
			CacheDirWazero: t.TempDir(),
			WorkspaceDir:   t.TempDir(),
			WorkingDir:     t.TempDir(),
		}
	}

	cell := func(t *testing.T, ws workspaces.Context, toc digest.Digest) error {
		return Cell(t.Context(), ws, uuid.Nil.String(), uuid.Nil.String(), svc, http.DefaultClient, wasm, image, toc)
	}

	t.Run("runs the module with the egiso as its root", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, cell(t, ws, toc))

		b, err := os.ReadFile(filepath.Join(ws.WorkingDir, "hostname"))
		require.NoError(t, err)
		require.Equal(t, "egd\n", string(b))
	})

	t.Run("removes the upper after the run", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, cell(t, ws, toc))

		matches, err := filepath.Glob(filepath.Join(ws.RuntimeDir, "cell.*"))
		require.NoError(t, err)
		require.Empty(t, matches)
	})

	t.Run("refuses an egiso whose table of contents does not match", func(t *testing.T) {
		require.ErrorContains(t, cell(t, workspace(t), digest.FromString("other")), "unable to verify the egiso")
	})
}
