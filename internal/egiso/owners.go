package egiso

import (
	"io/fs"
	"maps"
	"os/user"
	"strconv"
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
// or a host directory has, to the virtual users the environment exposes them as. An id maps by its
// number or by the name its source gives it. Sources disagree on their ids, and an environment has
// fewer users than its sources, so many ids map to one virtual user: by default 0 maps to Privileged
// and every other id to Default.
type Owners struct {
	privileged string
	standard   string
	ids        map[int]string
	names      map[string]string
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

// Name exposes what the id its source names name owns as the virtual user. A Map of the id's number
// takes precedence.
func (t Owners) Name(name string, virtual string) Owners {
	t.names = maps.Clone(t.names)
	if t.names == nil {
		t.names = map[string]string{}
	}
	t.names[name] = virtual
	return t
}

// Lookup returns the virtual user id, which its source names name, is exposed as.
func (t Owners) Lookup(id int, name string) string {
	if virtual, ok := t.ids[id]; ok {
		return virtual
	}
	if virtual, ok := t.names[name]; ok && name != "" {
		return virtual
	}
	if id == 0 {
		return t.privileged
	}
	return t.standard
}

// identity is who owns a file in its source: the uid and gid, and the names the source gives them.
type identity struct {
	uid   int
	gid   int
	user  string
	group string
}

// source returns who owns a file of src: the owner an archive's table of contents records, named by
// the archive, or a host file's owner, named by the host's user database.
func source(src fs.FS, info fs.FileInfo) (id identity, ok bool, err error) {
	switch sys := info.Sys().(type) {
	case *estargz.TOCEntry:
		id = identity{uid: sys.UID, gid: sys.GID}
		id.user, id.group, err = src.(*Archive).owner(sys)
		return id, true, err
	case *syscall.Stat_t:
		id = identity{uid: int(sys.Uid), gid: int(sys.Gid)}
		if u, cause := user.LookupId(strconv.Itoa(id.uid)); cause == nil {
			id.user = u.Username
		}
		if g, cause := user.LookupGroupId(strconv.Itoa(id.gid)); cause == nil {
			id.group = g.Name
		}
		return id, true, nil
	default:
		return id, false, nil
	}
}
