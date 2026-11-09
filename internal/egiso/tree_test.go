package egiso_test

import (
	"archive/tar"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestOwners(t *testing.T) {
	t.Run("exposes root as privileged and every other id as the default user", func(t *testing.T) {
		o := egiso.NewOwners()
		require.Equal(t, egiso.Privileged, o.Lookup(0, ""))
		require.Equal(t, egiso.Default, o.Lookup(1000, ""))
		require.Equal(t, egiso.Default, o.Lookup(5001, ""))
	})

	t.Run("exposes every id as one virtual user", func(t *testing.T) {
		o := egiso.NewOwners().Privileged(egiso.Default).Standard(egiso.Default)
		require.Equal(t, egiso.Default, o.Lookup(0, ""))
		require.Equal(t, egiso.Default, o.Lookup(1000, ""))
	})

	t.Run("maps many ids to one virtual user", func(t *testing.T) {
		o := egiso.NewOwners().Map(1001, "bob").Map(1002, "bob")
		require.Equal(t, "bob", o.Lookup(1001, ""))
		require.Equal(t, "bob", o.Lookup(1002, ""))
		require.Equal(t, egiso.Default, o.Lookup(1003, ""))
	})

	t.Run("maps an id over the privileged default", func(t *testing.T) {
		o := egiso.NewOwners().Map(0, egiso.Default)
		require.Equal(t, egiso.Default, o.Lookup(0, ""))
	})

	t.Run("maps an id by the name its source gives it", func(t *testing.T) {
		o := egiso.NewOwners().Name("bob", "carol")
		require.Equal(t, "carol", o.Lookup(1001, "bob"))
		require.Equal(t, egiso.Default, o.Lookup(1002, "alice"))
		require.Equal(t, egiso.Default, o.Lookup(1003, ""))
	})

	t.Run("maps an id by its number over its name", func(t *testing.T) {
		o := egiso.NewOwners().Name("bob", "carol").Map(1001, "alice")
		require.Equal(t, "alice", o.Lookup(1001, "bob"))
	})

	t.Run("never maps an id its source does not name by the empty name", func(t *testing.T) {
		o := egiso.NewOwners().Name("", "carol")
		require.Equal(t, egiso.Default, o.Lookup(1001, ""))
	})

	t.Run("leaves the owners it was derived from unchanged", func(t *testing.T) {
		base := egiso.NewOwners().Map(1001, "bob")
		derived := base.Map(1001, "alice").Standard("carol").Name("dave", "erin")
		require.Equal(t, "bob", base.Lookup(1001, ""))
		require.Equal(t, egiso.Default, base.Lookup(1002, ""))
		require.Equal(t, "alice", derived.Lookup(1001, ""))
		require.Equal(t, "carol", derived.Lookup(1002, ""))
		require.Equal(t, egiso.Default, base.Lookup(1004, "dave"))
		require.Equal(t, "erin", derived.Lookup(1004, "dave"))
	})
}

// archive builds an egiso holding the entries, with the uids and gids they give. A file holds its
// own name, except the account files, which name 0 and 1000.
func archive(t *testing.T, entries ...*tar.Header) *egiso.Archive {
	var (
		buf      bytes.Buffer
		out      bytes.Buffer
		accounts = map[string]string{
			"etc/passwd": "root:x:0:0:root:/root:/bin/sh\negd:x:1000:1000::/home/egd:/bin/sh\n",
			"etc/group":  "root:x:0:\negd:x:1000:\n",
		}
	)

	tw := tar.NewWriter(&buf)
	for _, h := range entries {
		h.ModTime = time.Unix(1700000000, 0)
		body := []byte(langx.FirstNonZero(accounts[h.Name], h.Name))
		if h.Typeflag != tar.TypeReg {
			body = nil
		}
		h.Size = int64(len(body))
		require.NoError(t, tw.WriteHeader(h))
		_, err := tw.Write(body)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	toc, err := egiso.Build(section(buf.Bytes()), &out)
	require.NoError(t, err)
	a, err := egiso.New(section(out.Bytes()), toc)
	require.NoError(t, err)
	return a
}

// hosted is a stand in for a host directory whose files have owners the test can not create.
func hosted(owners map[string]int) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, uid := range owners {
		fsys[name] = &fstest.MapFile{Data: []byte(name), Mode: 0644, Sys: &syscall.Stat_t{Uid: uint32(uid), Gid: uint32(uid)}}
	}
	return fsys
}

func TestTree(t *testing.T) {
	// the root image's default user is 1000; the tools image calls its users 5000 and 5001.
	root := archive(t,
		&tar.Header{Typeflag: tar.TypeDir, Name: "etc/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/passwd", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/group", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeDir, Name: "home/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeDir, Name: "home/egd/", Mode: 0700, Uid: 1000, Gid: 1000},
		&tar.Header{Typeflag: tar.TypeReg, Name: "home/egd/notes", Mode: 0600, Uid: 1000, Gid: 1000},
	)
	tools := archive(t,
		&tar.Header{Typeflag: tar.TypeDir, Name: "bin/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeReg, Name: "bin/build", Mode: 0755, Uid: 5000, Gid: 5000, Uname: "builder", Gname: "builders"},
		&tar.Header{Typeflag: tar.TypeReg, Name: "bin/test", Mode: 0755, Uid: 5001, Gid: 5001},
	)
	shared := hosted(map[string]int{"a": 0, "b": 1001, "c": 1002, "d": 1003, "e": 1004})

	host := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(host, "cache"), []byte("cached"), 0600))
	hostroot, err := os.OpenRoot(host)
	require.NoError(t, err)
	t.Cleanup(func() { hostroot.Close() })

	tree := egiso.Tree{}.
		Mount(".", root, egiso.NewOwners(), egiso.NewOwners()).
		Mount("opt/tools", tools, egiso.NewOwners(), egiso.NewOwners()).
		Mount("srv/shared", shared, egiso.NewOwners().Map(1001, "bob"), egiso.NewOwners()).
		Mount("home/egd/.cache", hostroot.FS(), egiso.NewOwners(), egiso.NewOwners())

	owner := func(t *testing.T, name string) (string, string) {
		user, group, err := tree.Owner(name)
		require.NoError(t, err)
		return user, group
	}

	t.Run("is a valid fs.FS", func(t *testing.T) {
		require.NoError(t, fstest.TestFS(tree, "etc/hostname", "home/egd/notes", "opt/tools/bin/build", "srv/shared/b", "home/egd/.cache/cache"))
	})

	t.Run("exposes an image's root files as privileged", func(t *testing.T) {
		user, group := owner(t, "etc/hostname")
		require.Equal(t, egiso.Privileged, user)
		require.Equal(t, egiso.Privileged, group)
	})

	t.Run("exposes two images' different users for the same role as the default user", func(t *testing.T) {
		user, _ := owner(t, "home/egd/notes")
		require.Equal(t, egiso.Default, user)

		user, group := owner(t, "opt/tools/bin/build")
		require.Equal(t, egiso.Default, user)
		require.Equal(t, egiso.Default, group)

		user, _ = owner(t, "opt/tools/bin/test")
		require.Equal(t, egiso.Default, user)
	})

	t.Run("exposes a directory's many owners through its own map", func(t *testing.T) {
		expected := map[string]string{"a": egiso.Privileged, "b": "bob", "c": egiso.Default, "d": egiso.Default, "e": egiso.Default}
		for name, virtual := range expected {
			user, _ := owner(t, "srv/shared/"+name)
			require.Equal(t, virtual, user, name)
		}
	})

	t.Run("exposes a host directory's files as the default user", func(t *testing.T) {
		user, _ := owner(t, "home/egd/.cache/cache")
		require.Equal(t, egiso.Default, user)

		b, err := fs.ReadFile(tree, "home/egd/.cache/cache")
		require.NoError(t, err)
		require.Equal(t, "cached", string(b))
	})

	t.Run("keeps a host directory's symlinks inside it", func(t *testing.T) {
		escaping := t.TempDir()
		require.NoError(t, os.Symlink("/etc", filepath.Join(escaping, "escape")))
		r, err := os.OpenRoot(escaping)
		require.NoError(t, err)
		defer r.Close()

		_, err = fs.ReadFile(tree.Mount("srv/escaping", r.FS(), egiso.NewOwners(), egiso.NewOwners()), "srv/escaping/escape/hosts")
		require.ErrorContains(t, err, "path escapes from parent")
	})

	t.Run("lists a mount point in the directory that holds it", func(t *testing.T) {
		entries, err := fs.ReadDir(tree, "home/egd")
		require.NoError(t, err)
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		require.Equal(t, []string{".cache", "notes"}, names)
	})

	t.Run("provides the directories leading to a mount point no image has", func(t *testing.T) {
		info, err := fs.Stat(tree, "opt")
		require.NoError(t, err)
		require.True(t, info.IsDir())

		user, group := owner(t, "opt")
		require.Equal(t, egiso.Privileged, user)
		require.Equal(t, egiso.Privileged, group)

		entries, err := fs.ReadDir(tree, ".")
		require.NoError(t, err)
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		require.Equal(t, []string{"etc", "home", "opt", "srv"}, names)
	})

	t.Run("exposes an image's owners by the names its /etc/passwd and /etc/group give them", func(t *testing.T) {
		named := tree.Mount(".", root, egiso.NewOwners().Name("egd", "bob"), egiso.NewOwners().Name("egd", "carol"))
		user, group, err := named.Owner("home/egd/notes")
		require.NoError(t, err)
		require.Equal(t, "bob", user)
		require.Equal(t, "carol", group)

		user, _, err = named.Owner("etc/hostname")
		require.NoError(t, err)
		require.Equal(t, egiso.Privileged, user)
	})

	t.Run("exposes an image's owners by the names its table of contents records", func(t *testing.T) {
		named := tree.Mount("opt/tools", tools, egiso.NewOwners().Name("builder", "bob"), egiso.NewOwners().Name("builders", "carol"))
		user, group, err := named.Owner("opt/tools/bin/build")
		require.NoError(t, err)
		require.Equal(t, "bob", user)
		require.Equal(t, "carol", group)

		user, _, err = named.Owner("opt/tools/bin/test")
		require.NoError(t, err)
		require.Equal(t, egiso.Default, user)
	})

	t.Run("exposes a host directory's owners by the names the host gives them", func(t *testing.T) {
		named := tree.Mount("srv/shared", shared, egiso.NewOwners().Name("root", "bob"), egiso.NewOwners())
		user, _, err := named.Owner("srv/shared/a")
		require.NoError(t, err)
		require.Equal(t, "bob", user)
	})

	t.Run("exposes every file of a mount as one virtual user", func(t *testing.T) {
		single := tree.Mount("srv/shared", shared, egiso.NewOwners().Privileged("bob").Standard("bob"), egiso.NewOwners())
		for _, name := range []string{"a", "b", "c"} {
			user, _, err := single.Owner("srv/shared/" + name)
			require.NoError(t, err)
			require.Equal(t, "bob", user)
		}
	})

	t.Run("reports a missing file", func(t *testing.T) {
		_, err := tree.Open("opt/missing")
		require.ErrorIs(t, err, fs.ErrNotExist)
		_, err = tree.Open("etc/missing")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("refuses an invalid path", func(t *testing.T) {
		_, err := tree.Open("/etc")
		require.ErrorIs(t, err, fs.ErrInvalid)
	})
}
