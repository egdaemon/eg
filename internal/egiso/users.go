package egiso

import (
	"bytes"
	"io"
	"io/fs"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/moby/sys/user"
)

// The virtual users eg runs commands as. An archive maps each to an account of its own, so neither
// needs an account on the host.
const (
	Privileged = "root"
	Default    = eg.DefaultUsername
)

// Account is an account an archive defines, which a virtual user maps to.
type Account struct {
	Name   string
	UID    int
	GID    int
	Groups []int
	Home   string
}

// Users maps virtual users to the accounts an archive defines.
type Users struct {
	accounts map[string]Account
	owners   map[int]string
}

// Users maps the virtual users to the accounts the archive defines in its own /etc/passwd and
// /etc/group, never the host's: Privileged to root, Default to egd, and each local user to the
// account of the same name. Every one of those accounts must exist in the archive. The archive's
// ownership comes from its table of contents alone, so the archive file itself only has to be
// readable by the host user that opens it.
func (t *Archive) Users(local ...string) (_ *Users, err error) {
	var (
		passwd []byte
		group  []byte
		u      *user.ExecUser
	)

	if passwd, err = fs.ReadFile(t, "etc/passwd"); err != nil {
		return nil, errorsx.Wrap(err, "unable to read the archive's accounts")
	}
	if group, err = fs.ReadFile(t, "etc/group"); err != nil && !errorsx.Is(err, fs.ErrNotExist) {
		return nil, errorsx.Wrap(err, "unable to read the archive's groups")
	}

	users := &Users{accounts: map[string]Account{}, owners: map[int]string{}}
	for _, name := range append([]string{Privileged, Default}, local...) {
		if u, err = user.GetExecUser(name, nil, bytes.NewReader(passwd), groupreader(group)); err != nil {
			return nil, errorsx.Wrapf(err, "the archive has no account for %s", name)
		}

		users.accounts[name] = Account{Name: name, UID: u.Uid, GID: u.Gid, Groups: u.Sgids, Home: u.Home}
		users.owners[u.Uid] = name
	}

	return users, nil
}

// Lookup returns the account the virtual user maps to.
func (t *Users) Lookup(virtual string) (Account, error) {
	a, ok := t.accounts[virtual]
	if !ok {
		return Account{}, errorsx.Errorf("%s is not mapped to an account of the archive", virtual)
	}
	return a, nil
}

// Owner returns the virtual user that owns a file of the archive, by the uid its table of contents
// records.
func (t *Users) Owner(info fs.FileInfo) (string, error) {
	e, ok := info.Sys().(*estargz.TOCEntry)
	if !ok {
		return "", errorsx.Errorf("%s is not a file of an archive", info.Name())
	}

	virtual, ok := t.owners[e.UID]
	if !ok {
		return "", errorsx.Errorf("%s is owned by uid %d, which no virtual user maps to", info.Name(), e.UID)
	}
	return virtual, nil
}

// groupreader reads an archive's /etc/group, a missing one has no groups.
func groupreader(group []byte) io.Reader {
	if group == nil {
		return nil
	}
	return bytes.NewReader(group)
}
