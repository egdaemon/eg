package compute

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/opencontainers/go-digest"
)

// machine runs the modules in a machine booted from the egiso, in place of the root container.
func (t local) machine(ctx context.Context, c *http.Client, ws workspaces.Context, modules ...transpile.Compiled) (err error) {
	var (
		m *runners.Machine
	)

	if m, err = runners.NewMachine(ctx, c, ws, t.Egiso, digest.Digest(t.TOC), log.Writer(),
		egvm.OptionMountReadOnly(filepath.Join(ws.Root, ws.BuildDir, ws.Module, eg.ModuleDir), eg.DefaultMountRoot(eg.RuntimeDirectory, ws.Module, eg.ModuleDir)),
	); err != nil {
		return err
	}
	defer func() { err = errorsx.Compact(err, m.Close()) }()

	for _, module := range modules {
		if err = m.Module(ctx, module.Path, nil, egvm.ExecOptionStdin(os.Stdin), egvm.ExecOptionStdout(log.Writer()), egvm.ExecOptionStderr(log.Writer())); err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
