package egiso

import (
	"io/fs"
	"maps"
	"syscall"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/egdaemon/eg"
)

// The virtual users of an environment. Nearly every environment needs only these two permission
// levels, and the owners of every mount map onto them.
const (
	Privileged = "root"
	Default    = eg.DefaultUsername
)

// Owners maps the ids that own a mount's files in their source, the uids or gids an archive records
// or a host directory has, to the virtual users the environment exposes them as. Sources disagree on
// their ids, and an environment has fewer users than its sources, so many ids map to one virtual
// user: by default 0 maps to Privileged and every other id to Default.
type Owners struct {
	privileged string
	standard   string
	ids        map[int]string
}

// NewOwners maps 0 to Privileged and every other id to Default.
func NewOwners() Owners {
	return Owners{privileged: Privileged, standard: Default}
}

// Privileged sets the virtual user that owns what id 0 owns in the source.
func (t Owners) Privileged(virtual string) Owners {
	t.privileged = virtual
	return t
}

// Standard sets the virtual user that owns what every unmapped id other than 0 owns in the source.
func (t Owners) Standard(virtual string) Owners {
	t.standard = virtual
	return t
}

// Map exposes what id owns in the source as the virtual user.
func (t Owners) Map(id int, virtual string) Owners {
	t.ids = maps.Clone(t.ids)
	if t.ids == nil {
		t.ids = map[int]string{}
	}
	t.ids[id] = virtual
	return t
}

// Lookup returns the virtual user id is exposed as.
func (t Owners) Lookup(id int) string {
	if virtual, ok := t.ids[id]; ok {
		return virtual
	}
	if id == 0 {
		return t.privileged
	}
	return t.standard
}

// source returns the uid and gid that own a file in its source: the owner an archive's table of
// contents records, or a host file's owner.
func source(info fs.FileInfo) (uid int, gid int, ok bool) {
	switch sys := info.Sys().(type) {
	case *estargz.TOCEntry:
		return sys.UID, sys.GID, true
	case *syscall.Stat_t:
		return int(sys.Uid), int(sys.Gid), true
	default:
		return 0, 0, false
	}
}
