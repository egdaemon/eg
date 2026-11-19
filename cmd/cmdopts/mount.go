package cmdopts

import (
	"io/fs"
	"strings"
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

// Volume renders the mount as a podman volume specification: source:destination[:ro|rw].
func (t Mount) Volume() string {
	parts := []string{t.Source, t.Destination}
	if t.Access != AccessDefault {
		parts = append(parts, string(t.Access))
	}

	return strings.Join(parts, ":")
}
