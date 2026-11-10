package workspaces

import (
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/fsx"
)

// RemapPassthrough returns the path as is. correct when the paths of the module are valid where they are used, for
// example the interpreter and the module sharing a container.
func RemapPassthrough(s string) string {
	return s
}

// RemapBaremetal returns the function that maps the absolute paths of a baremetal module, which are those of the container,
// to the host. relative paths are returned as is.
func RemapBaremetal(ws Context) func(string) string {
	return func(s string) (n string) {
		old := s
		defer func() {
			debugx.Println("remapped", ws.RuntimeDir, old, "->", n)
		}()

		// baremetal workloads are provided the host directories, which need no remapping.
		if _, ok := fsx.Relative(ws.Root, s); ok {
			return s
		}

		// the host directories mirror those of the container, resolve the paths of the container within the root.
		// the runtime directory is the one outside of the root directory of the container, see the TODO in egenv.go.
		for _, guest := range []string{eg.DefaultRootDirectory(), eg.DefaultMountRoot()} {
			if rel, ok := fsx.Relative(guest, s); ok {
				return filepath.Join(ws.Root, rel)
			}
		}

		return s
	}
}

// Remap returns the function that maps the paths of a baremetal module to the host. every path is within the root,
// a path that isn't is mapped under it, nothing resolves outside of the root. relative paths are mapped to the working directory,
// absolute paths of the container are mapped within the root.
func Remap(ws Context) func(string) string {
	return func(s string) (n string) {
		old := s
		defer func() {
			debugx.Println("remapped", ws.RuntimeDir, old, "->", n)
		}()

		if !filepath.IsAbs(s) {
			s = filepath.Join(ws.WorkingDir, s)
		}

		// baremetal workloads are provided the host directories, which need no remapping.
		if _, ok := fsx.Relative(ws.Root, s); ok {
			return s
		}

		// the host directories mirror those of the container, resolve the paths of the container within the root.
		// the runtime directory is the one outside of the root directory of the container, see the TODO in egenv.go.
		for _, guest := range []string{eg.DefaultRootDirectory(), eg.DefaultMountRoot()} {
			if rel, ok := fsx.Relative(guest, s); ok {
				return filepath.Join(ws.Root, rel)
			}
		}

		// everything else is placed under the root. cleaning a rooted path removes the elements that would leave it.
		return filepath.Join(ws.Root, filepath.Clean(filepath.Join("/", s)))
	}
}
