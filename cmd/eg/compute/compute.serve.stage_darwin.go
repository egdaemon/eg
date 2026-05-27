//go:build darwin

package compute

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
)

// stageHostDylibs walks the otool -L output of bin (recursively) and copies
// every /opt/homebrew dependency into dstDir keyed by its basename. Walks
// transitively so e.g. libgpgme's own deps (libassuan, libgpg-error) land
// alongside it. Apple-Silicon Homebrew only; Intel /usr/local/ is not
// scanned because the macvm runner is darwin/arm64.
func stageHostDylibs(bin, dstDir string) error {
	seen := map[string]bool{}
	var walk func(string) error
	walk = func(path string) error {
		out, err := exec.Command("otool", "-L", path).Output()
		if err != nil {
			return errorsx.Wrapf(err, "otool -L %s", path)
		}
		for _, line := range strings.Split(string(out), "\n") {
			// otool -L emits the inspected file's own install-name as the
			// first line ending in ":" — only dependency lines carry the
			// "(compatibility version ...)" suffix, so gate on that.
			if !strings.Contains(line, "(compatibility version") {
				continue
			}
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "/opt/homebrew/") {
				continue
			}
			lib := strings.Fields(line)[0]
			if seen[lib] {
				continue
			}
			seen[lib] = true
			dst := filepath.Join(dstDir, filepath.Base(lib))
			if err := copyExecutable(lib, dst); err != nil {
				return err
			}
			if err := walk(lib); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(bin)
}
