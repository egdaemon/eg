package egiso_test

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/stretchr/testify/require"
)

func TestUpper(t *testing.T) {
	image := archive(t,
		&tar.Header{Typeflag: tar.TypeDir, Name: "etc/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeDir, Name: "etc/apt/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/apt/sources", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeDir, Name: "home/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeDir, Name: "home/egd/", Mode: 0700, Uid: 1000, Gid: 1000},
		&tar.Header{Typeflag: tar.TypeReg, Name: "home/egd/notes", Mode: 0600, Uid: 1000, Gid: 1000},
		&tar.Header{Typeflag: tar.TypeReg, Name: "usr/bin/tool", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeSymlink, Name: "usr/bin/alias", Linkname: "tool", Mode: 0777},
	)

	// upper layers a fresh upper over the image, with a host directory bound at home/egd/.cache.
	upper := func(t *testing.T) (*egiso.Upper, string) {
		host := t.TempDir()
		cache, err := os.OpenRoot(host)
		require.NoError(t, err)
		t.Cleanup(func() { cache.Close() })
		dir, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() { dir.Close() })

		tree := egiso.Tree{}.
			Mount(".", image, egiso.NewOwners(), egiso.NewOwners()).
			Bind("home/egd/.cache", cache, egiso.NewOwners(), egiso.NewOwners())
		return egiso.NewUpper(tree, dir), host
	}

	read := func(t *testing.T, u *egiso.Upper, name string) string {
		f, err := u.Open(name)
		require.NoError(t, err)
		defer f.Close()
		b, err := io.ReadAll(f)
		require.NoError(t, err)
		return string(b)
	}

	write := func(t *testing.T, u *egiso.Upper, name string, flag int, data string) {
		f, err := u.OpenFile(name, flag, 0644, egiso.Default, egiso.Default)
		require.NoError(t, err)
		_, err = f.WriteString(data)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}

	names := func(t *testing.T, u *egiso.Upper, name string) []string {
		entries, err := u.ReadDir(name)
		require.NoError(t, err)
		listed := []string{}
		for _, e := range entries {
			listed = append(listed, e.Name())
		}
		return listed
	}

	t.Run("reads the tree through an empty upper", func(t *testing.T) {
		u, _ := upper(t)
		require.Equal(t, "etc/hostname", read(t, u, "etc/hostname"))
		require.Equal(t, []string{"apt", "hostname"}, names(t, u, "etc"))
	})

	t.Run("copies a file of the tree up before changing it", func(t *testing.T) {
		u, _ := upper(t)
		write(t, u, "etc/hostname", os.O_WRONLY|os.O_TRUNC, "changed")
		require.Equal(t, "changed", read(t, u, "etc/hostname"))

		b, err := fs.ReadFile(image, "etc/hostname")
		require.NoError(t, err)
		require.Equal(t, "etc/hostname", string(b))

		st, err := u.Lstat("etc/hostname")
		require.NoError(t, err)
		require.Equal(t, fs.FileMode(0644), st.Mode())

		user, group, err := u.Owner("etc/hostname")
		require.NoError(t, err)
		require.Equal(t, egiso.Privileged, user)
		require.Equal(t, egiso.Privileged, group)

		user, _, err = u.Owner("etc")
		require.NoError(t, err)
		require.Equal(t, egiso.Privileged, user)
	})

	t.Run("creates a file owned by the virtual user that created it", func(t *testing.T) {
		u, _ := upper(t)
		write(t, u, "etc/apt/extra", os.O_WRONLY|os.O_CREATE|os.O_EXCL, "deb")

		user, group, err := u.Owner("etc/apt/extra")
		require.NoError(t, err)
		require.Equal(t, egiso.Default, user)
		require.Equal(t, egiso.Default, group)
		require.Equal(t, []string{"extra", "sources"}, names(t, u, "etc/apt"))

		_, err = u.OpenFile("etc/apt/extra", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644, egiso.Default, egiso.Default)
		require.ErrorIs(t, err, fs.ErrExist)
	})

	t.Run("refuses to create in a directory that does not exist", func(t *testing.T) {
		u, _ := upper(t)
		_, err := u.OpenFile("srv/data", os.O_WRONLY|os.O_CREATE, 0644, egiso.Default, egiso.Default)
		require.ErrorIs(t, err, fs.ErrNotExist)

		_, err = u.OpenFile("etc/hostname/data", os.O_WRONLY|os.O_CREATE, 0644, egiso.Default, egiso.Default)
		require.ErrorIs(t, err, syscall.ENOTDIR)
	})

	t.Run("hides a removed file of the tree", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Remove("etc/hostname"))

		_, err := u.Lstat("etc/hostname")
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.Equal(t, []string{"apt"}, names(t, u, "etc"))

		_, err = u.Lstat("etc/.wh.hostname")
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.ErrorIs(t, u.Remove("etc/hostname"), fs.ErrNotExist)
	})

	t.Run("shows a file created where one was removed", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Remove("etc/hostname"))
		write(t, u, "etc/hostname", os.O_WRONLY|os.O_CREATE|os.O_EXCL, "new")
		require.Equal(t, "new", read(t, u, "etc/hostname"))
		require.Equal(t, []string{"apt", "hostname"}, names(t, u, "etc"))
	})

	t.Run("refuses to remove a directory with entries", func(t *testing.T) {
		u, _ := upper(t)
		require.ErrorIs(t, u.Remove("etc/apt"), syscall.ENOTEMPTY)

		require.NoError(t, u.Remove("etc/apt/sources"))
		require.NoError(t, u.Remove("etc/apt"))
		require.Equal(t, []string{"hostname"}, names(t, u, "etc"))
	})

	t.Run("hides the tree's entries in a directory recreated where one was removed", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Remove("etc/apt/sources"))
		require.NoError(t, u.Remove("etc/apt"))
		require.NoError(t, u.Mkdir("etc/apt", 0750, egiso.Default, egiso.Default))
		require.Empty(t, names(t, u, "etc/apt"))

		st, err := u.Lstat("etc/apt")
		require.NoError(t, err)
		require.Equal(t, fs.ModeDir|0750, st.Mode())

		_, err = u.Lstat("etc/apt/sources")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("hides the tree's entries under a directory replaced by a file", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Remove("etc/apt/sources"))
		require.NoError(t, u.Remove("etc/apt"))
		write(t, u, "etc/apt", os.O_WRONLY|os.O_CREATE|os.O_EXCL, "file")

		_, err := u.Lstat("etc/apt/sources")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("renames a file of the tree", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Rename("etc/hostname", "etc/apt/hostname"))
		require.Equal(t, []string{"apt"}, names(t, u, "etc"))
		require.Equal(t, "etc/hostname", read(t, u, "etc/apt/hostname"))

		user, _, err := u.Owner("etc/apt/hostname")
		require.NoError(t, err)
		require.Equal(t, egiso.Privileged, user)
	})

	t.Run("renames a file over another", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Rename("etc/hostname", "etc/apt/sources"))
		require.Equal(t, "etc/hostname", read(t, u, "etc/apt/sources"))
		require.Equal(t, []string{"apt"}, names(t, u, "etc"))
	})

	t.Run("renames a directory it created with its entries", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Mkdir("srv", 0755, egiso.Default, egiso.Default))
		write(t, u, "srv/data", os.O_WRONLY|os.O_CREATE, "data")
		require.NoError(t, u.Rename("srv", "opt"))

		require.Equal(t, "data", read(t, u, "opt/data"))
		user, _, err := u.Owner("opt/data")
		require.NoError(t, err)
		require.Equal(t, egiso.Default, user)

		_, err = u.Lstat("srv")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("renames a directory of the tree with everything in it", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Remove("home/egd/notes"))
		write(t, u, "home/egd/todo", os.O_WRONLY|os.O_CREATE, "todo")
		require.NoError(t, u.Rename("home", "users"))

		require.Equal(t, "todo", read(t, u, "users/egd/todo"))
		require.Equal(t, []string{".cache", "todo"}, names(t, u, "users/egd"))
		_, err := u.Lstat("home")
		require.ErrorIs(t, err, fs.ErrNotExist)

		user, _, err := u.Owner("users/egd")
		require.NoError(t, err)
		require.Equal(t, egiso.Default, user)

		require.NoError(t, u.Rename("etc", "config"))
		require.Equal(t, "etc/apt/sources", read(t, u, "config/apt/sources"))
		require.Equal(t, []string{"apt", "hostname"}, names(t, u, "config"))
	})

	t.Run("refuses to rename a file over a directory", func(t *testing.T) {
		u, _ := upper(t)
		require.ErrorIs(t, u.Rename("etc/hostname", "etc/apt"), syscall.EISDIR)
		require.ErrorIs(t, u.Rename("etc/hostname", "home"), syscall.EISDIR)
	})

	t.Run("creates and reads a symlink", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Symlink("tool", "usr/bin/other", egiso.Default, egiso.Default))

		target, err := u.ReadLink("usr/bin/other")
		require.NoError(t, err)
		require.Equal(t, "tool", target)

		st, err := u.Lstat("usr/bin/other")
		require.NoError(t, err)
		require.Equal(t, fs.ModeSymlink, st.Mode().Type())

		target, err = u.ReadLink("usr/bin/alias")
		require.NoError(t, err)
		require.Equal(t, "tool", target)
	})

	t.Run("copies a symlink up as a symlink", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Rename("usr/bin/alias", "usr/bin/moved"))
		target, err := u.ReadLink("usr/bin/moved")
		require.NoError(t, err)
		require.Equal(t, "tool", target)
	})

	t.Run("changes the mode the environment sees, not the tree's", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Chmod("usr/bin/tool", 0700|fs.ModeSetuid))

		st, err := u.Lstat("usr/bin/tool")
		require.NoError(t, err)
		require.Equal(t, 0700|fs.ModeSetuid, st.Mode())

		entries, err := u.ReadDir("usr/bin")
		require.NoError(t, err)
		info, err := entries[1].Info()
		require.NoError(t, err)
		require.Equal(t, "tool", info.Name())
		require.Equal(t, 0700|fs.ModeSetuid, info.Mode())

		st, err = fs.Stat(image, "usr/bin/tool")
		require.NoError(t, err)
		require.Equal(t, fs.FileMode(0755), st.Mode())
	})

	t.Run("changes the virtual owner", func(t *testing.T) {
		u, _ := upper(t)
		require.NoError(t, u.Chown("home/egd/notes", "bob", ""))
		user, group, err := u.Owner("home/egd/notes")
		require.NoError(t, err)
		require.Equal(t, "bob", user)
		require.Equal(t, egiso.Default, group)
	})

	t.Run("writes within a bound host directory to that directory as the host user", func(t *testing.T) {
		u, host := upper(t)
		require.NoError(t, u.Mkdir("home/egd/.cache/go-build", 0755, egiso.Privileged, egiso.Privileged))
		write(t, u, "home/egd/.cache/go-build/entry", os.O_WRONLY|os.O_CREATE, "cached")

		b, err := os.ReadFile(filepath.Join(host, "go-build", "entry"))
		require.NoError(t, err)
		require.Equal(t, "cached", string(b))

		st, err := os.Stat(filepath.Join(host, "go-build", "entry"))
		require.NoError(t, err)
		require.Equal(t, uint32(os.Getuid()), st.Sys().(*syscall.Stat_t).Uid)

		user, _, err := u.Owner("home/egd/.cache/go-build/entry")
		require.NoError(t, err)
		require.Equal(t, egiso.Default, user)

		require.NoError(t, os.RemoveAll(filepath.Join(host, "go-build")))
	})

	t.Run("keeps a bound host directory's files with the host user", func(t *testing.T) {
		u, _ := upper(t)
		write(t, u, "home/egd/.cache/entry", os.O_WRONLY|os.O_CREATE, "cached")
		require.ErrorIs(t, u.Chown("home/egd/.cache/entry", egiso.Privileged, ""), syscall.EPERM)
		require.ErrorIs(t, u.Rename("home/egd/.cache/entry", "home/egd/entry"), syscall.EXDEV)
		require.ErrorIs(t, u.Remove("home/egd/.cache"), syscall.EBUSY)

		require.NoError(t, u.Rename("home/egd/.cache/entry", "home/egd/.cache/moved"))
		require.Equal(t, "cached", read(t, u, "home/egd/.cache/moved"))
		require.NoError(t, u.Remove("home/egd/.cache/moved"))
	})

	t.Run("lists a bound host directory in the directory that holds it", func(t *testing.T) {
		u, _ := upper(t)
		require.Equal(t, []string{".cache", "notes"}, names(t, u, "home/egd"))
	})

	t.Run("refuses whiteout names", func(t *testing.T) {
		u, _ := upper(t)
		_, err := u.OpenFile("etc/.wh.hostname", os.O_WRONLY|os.O_CREATE, 0644, egiso.Default, egiso.Default)
		require.ErrorIs(t, err, fs.ErrInvalid)
	})
}
