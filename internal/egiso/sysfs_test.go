package egiso_test

import (
	"archive/tar"
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/experimental/sysfs"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestSysFS(t *testing.T) {
	image := archive(t,
		&tar.Header{Typeflag: tar.TypeDir, Name: "etc/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeDir, Name: "etc/apt/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/apt/sources", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeDir, Name: "home/egd/", Mode: 0700, Uid: 1000, Gid: 1000},
		&tar.Header{Typeflag: tar.TypeReg, Name: "usr/bin/tool", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeSymlink, Name: "usr/bin/alias", Linkname: "tool", Mode: 0777},
		&tar.Header{Typeflag: tar.TypeSymlink, Name: "usr/bin/abs", Linkname: "/usr/bin/tool", Mode: 0777},
		&tar.Header{Typeflag: tar.TypeSymlink, Name: "usr/lib", Linkname: "../etc", Mode: 0777},
		&tar.Header{Typeflag: tar.TypeSymlink, Name: "usr/loop", Linkname: "loop", Mode: 0777},
	)

	// mounted serves a fresh upper over the image, with a host directory bound at home/egd/.cache.
	mounted := func(t *testing.T) (*egiso.SysFS, string, string) {
		host := t.TempDir()
		cache, err := os.OpenRoot(host)
		require.NoError(t, err)
		t.Cleanup(func() { cache.Close() })
		upperdir := t.TempDir()
		dir, err := os.OpenRoot(upperdir)
		require.NoError(t, err)
		t.Cleanup(func() { dir.Close() })

		tree := egiso.Tree{}.
			Mount(".", image, egiso.NewOwners(), egiso.NewOwners()).
			Bind("home/egd/.cache", cache, egiso.NewOwners(), egiso.NewOwners())
		return egiso.NewSysFS(egiso.NewUpper(tree, dir), egiso.Default, egiso.Default), host, upperdir
	}

	t.Run("resolves symlinks across the whole filesystem", func(t *testing.T) {
		fsys, _, _ := mounted(t)
		for _, name := range []string{"usr/bin/alias", "usr/bin/abs"} {
			st, errno := fsys.Stat(name)
			require.Zero(t, errno, name)
			require.Equal(t, fs.FileMode(0755), st.Mode, name)
		}

		st, errno := fsys.Lstat("usr/bin/abs")
		require.Zero(t, errno)
		require.Equal(t, fs.ModeSymlink, st.Mode.Type())

		st, errno = fsys.Stat("usr/lib/apt/sources")
		require.Zero(t, errno)
		require.Equal(t, fs.FileMode(0644), st.Mode)
	})

	t.Run("refuses a symlink loop", func(t *testing.T) {
		fsys, _, _ := mounted(t)
		_, errno := fsys.Stat("usr/loop")
		require.Equal(t, experimentalsys.ELOOP, errno)
	})

	t.Run("refuses to open a symlink without following it", func(t *testing.T) {
		fsys, _, _ := mounted(t)
		_, errno := fsys.OpenFile("usr/bin/alias", experimentalsys.O_RDONLY|experimentalsys.O_NOFOLLOW, 0)
		require.Equal(t, experimentalsys.ELOOP, errno)
	})

	t.Run("writes through a symlink to its target", func(t *testing.T) {
		fsys, _, upperdir := mounted(t)
		f, errno := fsys.OpenFile("usr/lib/hostname", experimentalsys.O_WRONLY|experimentalsys.O_TRUNC, 0)
		require.Zero(t, errno)
		_, errno = f.Write([]byte("changed"))
		require.Zero(t, errno)
		require.Zero(t, f.Close())

		b, err := os.ReadFile(filepath.Join(upperdir, "etc", "hostname"))
		require.NoError(t, err)
		require.Equal(t, "changed", string(b))
	})

	t.Run("maps errors to errnos", func(t *testing.T) {
		fsys, _, _ := mounted(t)
		_, errno := fsys.Stat("etc/missing")
		require.Equal(t, experimentalsys.ENOENT, errno)
		require.Equal(t, experimentalsys.ENOTEMPTY, fsys.Rmdir("etc/apt"))
		require.Equal(t, experimentalsys.ENOTDIR, fsys.Rmdir("etc/hostname"))
		require.Equal(t, experimentalsys.EISDIR, fsys.Unlink("etc/apt"))
		require.Equal(t, experimentalsys.EEXIST, fsys.Mkdir("etc", 0755))

		_, errno = fsys.OpenFile("etc", experimentalsys.O_WRONLY, 0)
		require.Equal(t, experimentalsys.EISDIR, errno)
		_, errno = fsys.OpenFile("etc/hostname", experimentalsys.O_RDONLY|experimentalsys.O_DIRECTORY, 0)
		require.Equal(t, experimentalsys.ENOTDIR, errno)
		_, errno = fsys.OpenFile("etc/hostname", experimentalsys.O_WRONLY|experimentalsys.O_CREAT|experimentalsys.O_EXCL, 0644)
		require.Equal(t, experimentalsys.EEXIST, errno)
	})

	t.Run("lists a directory a few entries at a time and rewinds", func(t *testing.T) {
		fsys, _, _ := mounted(t)
		d, errno := fsys.OpenFile("usr/bin", experimentalsys.O_RDONLY, 0)
		require.Zero(t, errno)
		defer d.Close()

		first, errno := d.Readdir(2)
		require.Zero(t, errno)
		rest, errno := d.Readdir(2)
		require.Zero(t, errno)
		require.Len(t, first, 2)
		require.Len(t, rest, 1)

		_, errno = d.Seek(0, 0)
		require.Zero(t, errno)
		all, errno := d.Readdir(-1)
		require.Zero(t, errno)
		require.Equal(t, append(first, rest...), all)
	})

	t.Run("sets times on a file of the image by copying it up", func(t *testing.T) {
		fsys, _, upperdir := mounted(t)
		require.Zero(t, fsys.Utimens("etc/hostname", experimentalsys.UTIME_OMIT, 1700000000000000000))
		st, err := os.Stat(filepath.Join(upperdir, "etc", "hostname"))
		require.NoError(t, err)
		require.Equal(t, int64(1700000000), st.ModTime().Unix())
	})

	guest := filepath.Join(t.TempDir(), "guest.wasm")
	cmd := exec.Command("go", "build", "-o", guest, "./.xfixtures/guest")
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("unable to build the wasip1 guest: %v\n%s", err, b)
	}
	code, err := os.ReadFile(guest)
	require.NoError(t, err)

	rt := wazero.NewRuntime(t.Context())
	t.Cleanup(func() { rt.Close(t.Context()) })
	wasi_snapshot_preview1.MustInstantiate(t.Context(), rt)
	compiled, err := rt.CompileModule(t.Context(), code)
	require.NoError(t, err)

	// run runs one operation of the guest with fsys as its root, returning what it printed.
	run := func(t *testing.T, fsys *egiso.SysFS, args ...string) (string, error) {
		var (
			out bytes.Buffer
		)
		config := wazero.NewModuleConfig().
			WithName("").
			WithArgs(append([]string{"guest"}, args...)...).
			WithStdout(&out).
			WithStderr(&out).
			WithFSConfig(wazero.NewFSConfig().(sysfs.FSConfig).WithSysFSMount(fsys, "/"))
		_, err := rt.InstantiateModule(t.Context(), compiled, config)
		return out.String(), err
	}

	succeeds := func(t *testing.T, fsys *egiso.SysFS, args ...string) string {
		out, err := run(t, fsys, args...)
		require.NoError(t, err, out)
		return out
	}

	t.Run("guest reads the image", func(t *testing.T) {
		fsys, _, _ := mounted(t)
		require.Equal(t, "etc/hostname", succeeds(t, fsys, "cat", "/etc/hostname"))
		require.Equal(t, "usr/bin/tool", succeeds(t, fsys, "cat", "/usr/bin/abs"))
		require.Equal(t, "bin", succeeds(t, fsys, "pread", "/usr/bin/tool", "4", "3"))
		require.Equal(t, "true\n", succeeds(t, fsys, "regular", "/usr/bin/alias"))
		require.Equal(t, "/usr/bin/tool\n", succeeds(t, fsys, "readlink", "/usr/bin/abs"))
		require.Equal(t, ".cache\n", succeeds(t, fsys, "ls", "/home/egd"))
	})

	t.Run("guest changes the image's files without changing the image", func(t *testing.T) {
		fsys, _, _ := mounted(t)
		succeeds(t, fsys, "write", "/etc/hostname", "changed")
		require.Equal(t, "changed", succeeds(t, fsys, "cat", "/etc/hostname"))
		succeeds(t, fsys, "append", "/etc/hostname", "+more")
		require.Equal(t, "changed+more", succeeds(t, fsys, "cat", "/etc/hostname"))

		b, err := fs.ReadFile(image, "etc/hostname")
		require.NoError(t, err)
		require.Equal(t, "etc/hostname", string(b))
	})

	t.Run("guest creates, renames and removes", func(t *testing.T) {
		fsys, _, _ := mounted(t)
		succeeds(t, fsys, "mkdir", "/srv")
		succeeds(t, fsys, "write", "/srv/data", "data")
		succeeds(t, fsys, "mv", "/srv/data", "/srv/moved")
		succeeds(t, fsys, "ln", "moved", "/srv/link")
		require.Equal(t, "data", succeeds(t, fsys, "cat", "/srv/link"))
		require.Equal(t, "link\nmoved\n", succeeds(t, fsys, "ls", "/srv"))

		succeeds(t, fsys, "rm", "/etc/apt/sources")
		succeeds(t, fsys, "rm", "/etc/apt")
		require.Equal(t, "hostname\n", succeeds(t, fsys, "ls", "/etc"))

		_, err := run(t, fsys, "cat", "/etc/apt/sources")
		require.Error(t, err)
	})

	t.Run("guest writes to a bound host directory as the host user", func(t *testing.T) {
		fsys, host, upperdir := mounted(t)
		succeeds(t, fsys, "mkdir", "/home/egd/.cache/go-build")
		succeeds(t, fsys, "write", "/home/egd/.cache/go-build/entry", "cached")

		b, err := os.ReadFile(filepath.Join(host, "go-build", "entry"))
		require.NoError(t, err)
		require.Equal(t, "cached", string(b))

		_, err = os.Stat(filepath.Join(upperdir, "home", "egd", ".cache"))
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.NoError(t, os.RemoveAll(filepath.Join(host, "go-build")))
	})
}

func TestNewShare(t *testing.T) {
	// shared serves a fresh host directory alone.
	shared := func(t *testing.T) (*egiso.SysFS, string) {
		host := t.TempDir()
		dir, err := os.OpenRoot(host)
		require.NoError(t, err)
		t.Cleanup(func() { dir.Close() })
		return egiso.NewShare(dir), host
	}

	t.Run("writes to the host directory as the host user", func(t *testing.T) {
		fsys, host := shared(t)
		require.Zero(t, fsys.Mkdir("go-build", 0755))
		f, errno := fsys.OpenFile("go-build/entry", experimentalsys.O_CREAT|experimentalsys.O_WRONLY, 0644)
		require.Zero(t, errno)
		_, errno = f.Write([]byte("cached"))
		require.Zero(t, errno)
		require.Zero(t, f.Close())

		b, err := os.ReadFile(filepath.Join(host, "go-build", "entry"))
		require.NoError(t, err)
		require.Equal(t, "cached", string(b))
	})

	t.Run("exposes the host user's files as the default user's", func(t *testing.T) {
		fsys, host := shared(t)
		require.NoError(t, os.WriteFile(filepath.Join(host, "file"), nil, 0644))

		user, group, err := fsys.Upper().Owner("file")
		require.NoError(t, err)
		require.Equal(t, egiso.Default, user)
		require.Equal(t, egiso.Default, group)
	})

	t.Run("refuses to change an owner", func(t *testing.T) {
		fsys, host := shared(t)
		require.NoError(t, os.WriteFile(filepath.Join(host, "file"), nil, 0644))
		require.ErrorIs(t, fsys.Upper().Chown("file", egiso.Privileged, ""), syscall.EPERM)
	})

	t.Run("renames, links, lists and removes within the host directory", func(t *testing.T) {
		fsys, host := shared(t)
		require.NoError(t, os.WriteFile(filepath.Join(host, "file"), []byte("data"), 0644))
		require.Zero(t, fsys.Rename("file", "moved"))
		require.Zero(t, fsys.Link("moved", "linked"))
		require.Zero(t, fsys.Symlink("moved", "alias"))
		require.Zero(t, fsys.Chmod("moved", 0600))
		require.Zero(t, fsys.Utimens("moved", 0, 0))

		dir, errno := fsys.OpenFile(".", experimentalsys.O_RDONLY, 0)
		require.Zero(t, errno)
		entries, errno := dir.Readdir(-1)
		require.Zero(t, errno)
		require.Zero(t, dir.Close())
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name)
		}
		require.ElementsMatch(t, []string{"alias", "linked", "moved"}, names)

		require.Zero(t, fsys.Unlink("alias"))
		require.Zero(t, fsys.Unlink("linked"))
		require.Zero(t, fsys.Unlink("moved"))
		require.Zero(t, fsys.Mkdir("empty", 0755))
		require.Zero(t, fsys.Rmdir("empty"))

		remaining, err := os.ReadDir(host)
		require.NoError(t, err)
		require.Empty(t, remaining)
	})

	t.Run("sees what the host changes", func(t *testing.T) {
		fsys, host := shared(t)
		_, errno := fsys.Stat("later")
		require.Equal(t, experimentalsys.ENOENT, errno)
		require.NoError(t, os.WriteFile(filepath.Join(host, "later"), []byte("later"), 0644))
		st, errno := fsys.Stat("later")
		require.Zero(t, errno)
		require.Equal(t, int64(5), st.Size)
	})
}
