package interp

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/workspaces"
	"github.com/opencontainers/go-digest"
	"google.golang.org/grpc"
)

// Cell runs a module the way Remote does, with the egiso at location, pinned to toc, as its root in
// place of a container. The image sits under a writable upper kept in the workspace's runtime
// directory for the run, with the workspace's directories bound into it where the module sees them.
// WASI has no owners, so what the module creates belongs to the default user. The commands the
// module does not otherwise provide run in a machine booted over the same root on the first of
// them.
func Cell(ctx context.Context, ws workspaces.Context, aid string, runid string, svc grpc.ClientConnInterface, c *http.Client, module string, location string, toc digest.Digest, environ ...string) (err error) {
	var (
		archive *egiso.Archive
		dir     string
		upper   *os.Root
		bound   *os.Root
	)

	if archive, err = egiso.Open(ctx, c, location, toc); err != nil {
		return err
	}
	defer archive.Close()

	if dir, err = os.MkdirTemp(ws.RuntimeDir, "cell.*"); err != nil {
		return errorsx.Wrap(err, "unable to create the cell's upper")
	}
	defer os.RemoveAll(dir)

	if upper, err = os.OpenRoot(dir); err != nil {
		return errorsx.Wrap(err, "unable to open the cell's upper")
	}
	defer upper.Close()

	tree := egiso.Tree{}.Mount(".", archive, egiso.NewOwners(), egiso.NewOwners())
	for _, d := range workspace(ws) {
		if bound, err = os.OpenRoot(d.host); err != nil {
			return errorsx.Wrapf(err, "unable to bind %s into the cell", d.host)
		}
		defer bound.Close()
		tree = tree.Bind(strings.TrimPrefix(d.guest, "/"), bound, egiso.NewOwners(), egiso.NewOwners())
	}
	root := egiso.NewSysFS(egiso.NewUpper(tree, upper), egiso.Default, egiso.Default)

	m := &machine{root: root}
	defer func() { err = errorsx.Compact(err, m.Close()) }()

	return Remote(ctx, ws, aid, runid, svc, module, OptionEnviron(environ...), OptionRoot(root), OptionMachine(m.Exec))
}

// machine is a cell's machine, booted with the image in egvm.Directory on the first command run
// in it, and sharing the cell's root over NFS.
type machine struct {
	root    *egiso.SysFS
	once    sync.Once
	dir     string
	unmount func() error
	vm      *egvm.VM
	err     error
}

// Exec runs req in the machine, booting it first if it is not running.
func (t *machine) Exec(ctx context.Context, req *execproxy.ExecRequest, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int32, error) {
	t.once.Do(t.boot)
	if t.err != nil {
		return -1, t.err
	}
	return t.vm.Exec(ctx, req, stdin, stdout, stderr)
}

func (t *machine) boot() {
	if t.dir, t.err = os.MkdirTemp("", "egvm.*"); t.err != nil {
		return
	}
	if t.unmount, t.err = egvm.Mount(t.root, t.dir); t.err != nil {
		return
	}
	t.vm, t.err = egvm.Boot(egvm.New(egvm.Directory(), t.dir), os.Stderr)
}

// Close stops the machine, if it booted, and unmounts the root it shared.
func (t *machine) Close() (err error) {
	if t.vm != nil {
		err = t.vm.Close()
	}
	if t.unmount != nil {
		err = errorsx.Compact(err, t.unmount())
	}
	if t.dir != "" {
		err = errorsx.Compact(err, os.Remove(t.dir))
	}
	return err
}
