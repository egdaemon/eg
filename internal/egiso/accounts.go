package egiso

import (
	"bytes"
	"io"
	"io/fs"
	"maps"
	"slices"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/moby/sys/user"
)

// Account is an id of an archive: one its /etc/passwd or /etc/group defines, or one that owns its
// files, which is what a mount's Owners maps.
type Account struct {
	ID    int
	Name  string // from /etc/passwd or /etc/group, else the name the table of contents records
	Files int    // files, directories and links the id owns
}

// Accounts lists the archive's users and groups by id, for setting up the Owners of a mount.
func (t *Archive) Accounts() (users []Account, groups []Account, err error) {
	var (
		uids   = map[int]*Account{}
		gids   = map[int]*Account{}
		passwd []user.User
		group  []user.Group
	)

	account := func(ids map[int]*Account, id int) *Account {
		if a, ok := ids[id]; ok {
			return a
		}
		ids[id] = &Account{ID: id}
		return ids[id]
	}

	err = fs.WalkDir(t, ".", func(name string, d fs.DirEntry, walked error) error {
		if walked != nil {
			return walked
		}
		info, cause := d.Info()
		if cause != nil {
			return cause
		}
		e := info.Sys().(*estargz.TOCEntry)

		u := account(uids, e.UID)
		u.Files++
		u.Name = langx.FirstNonZero(u.Name, e.Uname)
		g := account(gids, e.GID)
		g.Files++
		g.Name = langx.FirstNonZero(g.Name, e.Gname)
		return nil
	})
	if err != nil {
		return nil, nil, errorsx.Wrap(err, "unable to read the archive's owners")
	}

	if passwd, err = defined(t, "etc/passwd", user.ParsePasswd); err != nil {
		return nil, nil, err
	}
	for _, u := range passwd {
		account(uids, u.Uid).Name = u.Name
	}

	if group, err = defined(t, "etc/group", user.ParseGroup); err != nil {
		return nil, nil, err
	}
	for _, g := range group {
		account(gids, g.Gid).Name = g.Name
	}

	return sorted(uids), sorted(gids), nil
}

// defined parses one of the archive's account files, a missing one defines nothing.
func defined[T any](archive *Archive, name string, parse func(io.Reader) ([]T, error)) ([]T, error) {
	b, err := fs.ReadFile(archive, name)
	if errorsx.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errorsx.Wrapf(err, "unable to read the archive's %s", name)
	}

	entries, err := parse(bytes.NewReader(b))
	if err != nil {
		return nil, errorsx.Wrapf(err, "unable to parse the archive's %s", name)
	}
	return entries, nil
}

func sorted(ids map[int]*Account) (accounts []Account) {
	for _, id := range slices.Sorted(maps.Keys(ids)) {
		accounts = append(accounts, *ids[id])
	}
	return accounts
}
