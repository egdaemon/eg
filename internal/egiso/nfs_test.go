package egiso_test

import (
	"archive/tar"
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/stretchr/testify/require"
)

func TestServeNFS(t *testing.T) {
	image := archive(t,
		&tar.Header{Typeflag: tar.TypeDir, Name: "etc/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeDir, Name: "etc/apt/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/apt/sources", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeDir, Name: "usr/bin/", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeReg, Name: "usr/bin/tool", Mode: 0755},
		&tar.Header{Typeflag: tar.TypeSymlink, Name: "usr/bin/abs", Linkname: "tool", Mode: 0777},
		&tar.Header{Typeflag: tar.TypeSymlink, Name: "usr/bin/dangling", Linkname: "missing", Mode: 0777},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/passwd", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeReg, Name: "etc/group", Mode: 0644},
		&tar.Header{Typeflag: tar.TypeDir, Name: "home/egd/", Mode: 0755, Uid: 1000, Gid: 1000},
		&tar.Header{Typeflag: tar.TypeReg, Name: "home/egd/file", Mode: 0644, Uid: 1000, Gid: 1000},
	)

	// mounted serves a fresh upper over the image, its owners exposed as users and groups, over NFS
	// and mounts it, returning the mount point and the upper's directory. the test is skipped when the
	// host refuses the mount.
	mounted := func(t *testing.T, users egiso.Owners, groups egiso.Owners, options ...egiso.NFSOption) (string, string) {
		upperdir := t.TempDir()
		dir, err := os.OpenRoot(upperdir)
		require.NoError(t, err)
		t.Cleanup(func() { dir.Close() })

		tree := egiso.Tree{}.Mount(".", image, users, groups)
		fsys := egiso.NewSysFS(egiso.NewUpper(tree, dir), egiso.Default, egiso.Default)

		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		go egiso.ServeNFS(l, fsys, options...)
		t.Cleanup(func() { l.Close() })

		port := l.Addr().(*net.TCPAddr).Port
		mnt := t.TempDir()
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("mount_nfs", "-o", fmt.Sprintf("vers=3,tcp,port=%d,mountport=%d,nolocks,noresvport,locallocks,soft,timeo=10", port, port), "127.0.0.1:/", mnt)
		default:
			cmd = exec.Command("mount", "-t", "nfs", "-o", fmt.Sprintf("vers=3,tcp,port=%d,mountport=%d,nolock,noresvport,soft", port, port), "127.0.0.1:/", mnt)
		}
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("unable to mount over nfs: %v\n%s", err, b)
		}
		t.Cleanup(func() {
			if b, err := exec.Command("umount", "-f", mnt).CombinedOutput(); err != nil {
				t.Logf("unable to unmount %s: %v\n%s", mnt, err, b)
			}
		})
		return mnt, upperdir
	}

	// sh runs a native shell command within the mount.
	sh := func(t *testing.T, mnt string, script string) string {
		ctx, done := context.WithTimeout(t.Context(), time.Minute)
		defer done()
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
		cmd.Dir = mnt
		b, err := cmd.CombinedOutput()
		require.NoError(t, err, string(b))
		return string(b)
	}

	t.Run("native processes read the image", func(t *testing.T) {
		mnt, _ := mounted(t, egiso.NewOwners(), egiso.NewOwners())
		require.Equal(t, "etc/hostname", sh(t, mnt, "cat etc/hostname"))
		require.Equal(t, "usr/bin/tool", sh(t, mnt, "cat usr/bin/abs"))
		require.Equal(t, "apt\ngroup\nhostname\npasswd\n", sh(t, mnt, "ls etc"))
	})

	t.Run("native processes change the image without changing it", func(t *testing.T) {
		mnt, upperdir := mounted(t, egiso.NewOwners(), egiso.NewOwners())
		sh(t, mnt, "printf changed > etc/hostname && mkdir -p srv/data && printf x > srv/data/f && mv srv/data/f srv/data/g && rm etc/apt/sources && rmdir etc/apt")
		require.Equal(t, "changed", sh(t, mnt, "cat etc/hostname"))
		require.Equal(t, "g\n", sh(t, mnt, "ls srv/data"))
		require.Equal(t, "group\nhostname\npasswd\n", sh(t, mnt, "ls etc"))

		b, err := os.ReadFile(filepath.Join(upperdir, "etc", "hostname"))
		require.NoError(t, err)
		require.Equal(t, "changed", string(b))
		b, err = fs.ReadFile(image, "etc/hostname")
		require.NoError(t, err)
		require.Equal(t, "etc/hostname", string(b))
	})

	t.Run("a symlink whose target is missing still holds its name", func(t *testing.T) {
		mnt, _ := mounted(t, egiso.NewOwners(), egiso.NewOwners())
		require.Contains(t, sh(t, mnt, "ln -s tool usr/bin/dangling 2>&1; true"), "File exists")
		require.Equal(t, "missing", sh(t, mnt, "readlink usr/bin/dangling | tr -d '\\n'"))
	})

	// owners prints the uid and gid of each named file in the mount.
	owners := func(t *testing.T, mnt string, names ...string) string {
		format := "-c '%u %g'"
		if runtime.GOOS == "darwin" {
			format = "-f '%u %g'"
		}
		return sh(t, mnt, "stat "+format+" "+strings.Join(names, " "))
	}

	t.Run("every file belongs to the host user by default", func(t *testing.T) {
		mnt, _ := mounted(t, egiso.NewOwners(), egiso.NewOwners())
		host := fmt.Sprintf("%d %d\n", os.Getuid(), os.Getgid())
		require.Equal(t, host+host, owners(t, mnt, "etc/hostname", "home/egd/file"))
	})

	t.Run("reports owners as the ids the accounts give them", func(t *testing.T) {
		users, groups, err := image.Accounts()
		require.NoError(t, err)
		mnt, _ := mounted(t, egiso.Identity(users), egiso.Identity(groups), egiso.NFSOptionAccounts(users, groups))
		sh(t, mnt, "printf x > created")
		require.Equal(t, "0 0\n1000 1000\n1000 1000\n", owners(t, mnt, "etc/hostname", "home/egd/file", "created"))
	})

	t.Run("reports an owner the accounts do not name as nobody", func(t *testing.T) {
		root := []egiso.Account{{ID: 0, Name: egiso.Privileged}}
		mnt, _ := mounted(t, egiso.NewOwners(), egiso.NewOwners(), egiso.NFSOptionAccounts(root, root))
		require.Equal(t, "0 0\n65534 65534\n", owners(t, mnt, "etc/hostname", "home/egd/file"))
	})

	t.Run("git works in the mount", func(t *testing.T) {
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git is not installed")
		}
		mnt, _ := mounted(t, egiso.NewOwners(), egiso.NewOwners())
		out := sh(t, mnt, "git init -q repo && cd repo && printf hello > a && git add a && git -c user.name=t -c user.email=t@t commit -q -m one && git log --format=%s")
		require.Equal(t, "one", strings.TrimSpace(out))
	})
}
