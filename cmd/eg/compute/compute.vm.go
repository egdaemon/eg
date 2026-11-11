package compute

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/runners"
)

// vm runs a workload as local does, in a machine booted from an egiso in place of podman's
// containers.
type vm struct {
	local
	Egiso string `name:"egiso" help:"path or https URL of the egiso the machine boots, with the digest of its table of contents as its fragment, built from the default root container when absent"`
}

func (t vm) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, hotswapbin *cmdopts.HotswapPath) error {
	return t.local.run(gctx, hotswapbin, t.machine(tlsc.DefaultClient()))
}

// machine runs the workload's modules in a machine booted from the egiso, fetched through c.
func (t vm) machine(c *http.Client) runner {
	return func(ctx context.Context, w workload) (err error) {
		var (
			m   *runners.Machine
			uri = t.Egiso
		)

		if err = w.environ(); err != nil {
			return err
		}

		if stringsx.Blank(uri) {
			if uri, err = runners.BuildRootEgiso(ctx, userx.DefaultCacheDirectory("egiso"), t.Dir, filepath.Join(w.ws.RuntimeDir, "Containerfile"), "--build-arg", envx.Format(eg.EnvBuildCommit, cmdopts.BuildCommit()), "--platform", podmanx.AutoPlatform(t.Arch)); err != nil {
				return err
			}
		}

		if m, err = runners.NewMachine(ctx, c, w.ws, uri, log.Writer(),
			egvm.OptionMountReadOnly(filepath.Join(w.ws.Root, w.ws.BuildDir, w.ws.Module, eg.ModuleDir), eg.DefaultMountRoot(eg.RuntimeDirectory, w.ws.Module, eg.ModuleDir)),
		); err != nil {
			return err
		}
		defer func() { err = errorsx.Compact(err, m.Close()) }()

		for _, module := range w.modules {
			if err = m.Module(ctx, module.Path, nil, egvm.ExecOptionStdin(os.Stdin), egvm.ExecOptionStdout(log.Writer()), egvm.ExecOptionStderr(log.Writer())); err != nil {
				return errorsx.Wrap(err, "module execution failed")
			}
		}

		return nil
	}
}
