package cmdopts

import (
	"io/fs"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/fsx"
)

// Access how a mounted directory may be used by the workload.
type Access string

const (
	// AccessDefault leaves the access to the container runtime, which mounts read write.
	AccessDefault Access = ""
	AccessRead    Access = "ro"
	AccessWrite   Access = "rw"
)

// Mount a host directory made available to the workload.
//
// Mode, User, and Group describe the mount point itself, not the files inside it. they are parsed and kept
// but not yet honored by podman.
type Mount struct {
	Source      string
	Destination string
	Mode        fs.FileMode
	User        string
	Group       string
	Access      Access
}

// Remapped the location of the destination within the root directory, when it is there. such a mount is mounted at
// the same location within the mount root, the runner remaps it to the destination so the unprivileged user has
// access to it, see Mounts.Remap.
func (t Mount) Remapped() (rel string, ok bool) {
	// the root directory itself is not remapped, it would be the mount root.
	rel, ok = fsx.Relative(eg.DefaultRootDirectory(), t.Destination)
	return rel, ok && rel != "."
}

// Mountpoint the location the mount is mounted at: the destination, or its location within the mount root when it
// is remapped.
func (t Mount) Mountpoint() string {
	if rel, ok := t.Remapped(); ok {
		return eg.DefaultMountRoot(rel)
	}

	return t.Destination
}

// Volume renders the mount as a podman volume specification: source:mountpoint[:ro|rw].
func (t Mount) Volume() string {
	parts := []string{t.Source, t.Mountpoint()}
	if t.Access != AccessDefault {
		parts = append(parts, string(t.Access))
	}

	return strings.Join(parts, ":")
}
