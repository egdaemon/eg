package egvm

import (
	"fmt"
	"net"
	"os/exec"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/errorsx"
)

// Mount serves fsys over NFS on a loopback port and mounts it at dir, so a machine can share it;
// macOS mounts NFS without privileges. unmount stops both.
func Mount(fsys *egiso.SysFS, dir string) (unmount func() error, err error) {
	var (
		l net.Listener
	)

	if l, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
		return nil, errorsx.Wrap(err, "unable to listen for nfs")
	}
	go egiso.ServeNFS(l, fsys)

	port := l.Addr().(*net.TCPAddr).Port
	options := fmt.Sprintf("vers=3,tcp,port=%d,mountport=%d,nolocks,noresvport,locallocks,soft,timeo=10", port, port)
	if b, err := exec.Command("mount_nfs", "-o", options, "127.0.0.1:/", dir).CombinedOutput(); err != nil {
		return nil, errorsx.Compact(errorsx.Wrapf(err, "unable to mount nfs at %s: %s", dir, b), l.Close())
	}

	return func() error {
		if b, err := exec.Command("umount", "-f", dir).CombinedOutput(); err != nil {
			return errorsx.Compact(errorsx.Wrapf(err, "unable to unmount %s: %s", dir, b), l.Close())
		}
		return l.Close()
	}, nil
}
