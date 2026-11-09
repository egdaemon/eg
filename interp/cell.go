package interp

import (
	"context"
	"net/http"
	"os"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/opencontainers/go-digest"
	"google.golang.org/grpc"
)

// Cell runs a module the way Remote does, with the egiso at location, pinned to toc, as its root in
// place of a container. The image sits under a writable upper kept in the workspace's runtime
// directory for the run; the workspace's directories mount over it as usual. WASI has no owners,
// so what the module creates belongs to the default user.
func Cell(ctx context.Context, ws workspaces.Context, aid string, runid string, svc grpc.ClientConnInterface, c *http.Client, module string, location string, toc digest.Digest, environ ...string) (err error) {
	var (
		archive *egiso.Archive
		dir     string
		upper   *os.Root
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
	root := egiso.NewSysFS(egiso.NewUpper(tree, upper), egiso.Default, egiso.Default)

	return Remote(ctx, ws, aid, runid, svc, module, OptionEnviron(environ...), OptionRoot(root))
}
