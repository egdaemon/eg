package compute

import (
	"io"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
)

// stageHostToolchain copies the running eg binary into runtimeDir so that
// downstream runners (podman containers, the macvm proxy) can mount it
// through a guest-visible shared filesystem. On darwin the binary's
// /opt/homebrew dylib closure is copied alongside it so dyld inside the
// guest resolves them by basename via DYLD_LIBRARY_PATH; elsewhere
// stageHostDylibs is a no-op.
func stageHostToolchain(runtimeDir string) (string, error) {
	src, err := os.Executable()
	if err != nil {
		return "", errorsx.Wrap(err, "locate running eg binary")
	}
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return "", errorsx.Wrap(err, "create runtime dir")
	}
	dst := filepath.Join(runtimeDir, eg.HostBin)
	if err := copyExecutable(src, dst); err != nil {
		return "", err
	}
	if err := stageHostDylibs(src, runtimeDir); err != nil {
		return "", errorsx.Wrap(err, "stage host dylibs")
	}
	return dst, nil
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return errorsx.Wrapf(err, "open %s", src)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return errorsx.Wrapf(err, "create %s", dst)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return errorsx.Wrapf(err, "copy %s -> %s", src, dst)
	}
	return out.Chmod(0o755)
}
