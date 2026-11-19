package cmdopts

import (
	"fmt"
	"os"
)

// Mounts are the flags for making host directories available to a workload.
// each is of the form source:destination[:mode[:user[:group]]], where mode, user, and group describe the mount
// point and not its contents.
type Mounts struct {
	Mount          []string `name:"mount" help:"mount a host directory into the workload, source:destination[:mode[:user[:group]]]" hidden:"true"`
	MountReadOnly  []string `name:"mount-read-only" help:"mount a host directory into the workload read only, source:destination[:mode[:user[:group]]]" hidden:"true"`
	MountReadWrite []string `name:"mount-read-write" help:"mount a host directory into the workload read write, source:destination[:mode[:user[:group]]]" hidden:"true"`
}

// Validate ensures every mount specification parses.
func (t Mounts) Validate() error {
	_, err := t.Specs()
	return err
}

// Specs parses the mounts in the order: default, read only, read write.
func (t Mounts) Specs() (mounts []Mount, err error) {
	for _, group := range []struct {
		specs  []string
		access Access
	}{
		{t.Mount, AccessDefault},
		{t.MountReadOnly, AccessRead},
		{t.MountReadWrite, AccessWrite},
	} {
		for _, spec := range group.specs {
			m, err := ParseMount(spec, group.access)
			if err != nil {
				return nil, err
			}

			mounts = append(mounts, m)
		}
	}

	return mounts, nil
}

// Volumes the mounts as podman volume specifications. podman reports a missing source as an opaque
// exit 125, so the source of every mount must exist.
func (t Mounts) Volumes() (volumes []string, err error) {
	mounts, err := t.Specs()
	if err != nil {
		return nil, err
	}

	for _, m := range mounts {
		if _, err = os.Stat(m.Source); err != nil {
			return nil, fmt.Errorf("unable to mount %s: %w", m.Source, err)
		}

		volumes = append(volumes, m.Volume())
	}

	return volumes, nil
}
