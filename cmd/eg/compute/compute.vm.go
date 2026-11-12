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
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
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

// machine runs modules in a machine booted from an egiso.
type machine struct {
	*runners.Machine
}

func (t machine) Do(ctx context.Context, m transpile.Compiled) error {
	return t.Module(ctx, m.Path, nil, egvm.ExecOptionStdin(os.Stdin), egvm.ExecOptionStdout(log.Writer()), egvm.ExecOptionStderr(log.Writer()))
}

// machine prepares a machine booted from the egiso, fetched through c, or from one of the default
// root container when no egiso is given.
func (t vm) machine(c *http.Client) newrunner {
	return func(ctx context.Context, ws workspaces.Context, _ uuid.UUID, _ string, _ *envx.Builder) (_ runner, err error) {
		var (
			m   *runners.Machine
			uri = t.Egiso
		)

		if stringsx.Blank(uri) {
			if uri, err = runners.BuildRootEgiso(ctx, userx.DefaultCacheDirectory("egiso"), t.Dir, filepath.Join(ws.RuntimeDir, "Containerfile"), "--build-arg", envx.Format(eg.EnvBuildCommit, cmdopts.BuildCommit()), "--platform", podmanx.AutoPlatform(t.Arch)); err != nil {
				return nil, err
			}
		}

		if m, err = runners.NewMachine(ctx, c, ws, uri, log.Writer(),
			egvm.OptionMountReadOnly(filepath.Join(ws.Root, ws.BuildDir, ws.Module, eg.ModuleDir), eg.DefaultMountRoot(eg.RuntimeDirectory, ws.Module, eg.ModuleDir)),
		); err != nil {
			return nil, err
		}

		return machine{Machine: m}, nil
	}
}
