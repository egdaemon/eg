package workloads

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	egcompute "github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/asynccompute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/jsonx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
)

type detectCmd struct {
	Roots   []string `arg:"" optional:"" name:"root" help:"root directories of the repositories to detect the workloads of" default:"${vars_eg_root_directory}"`
	JSONL   bool     `name:"jsonl" help:"print the workloads as json lines, one per way a workload can run, for tools such as editor extensions. workloads ignored by an .egrcignore are included, marked ignored"`
	Workers uint16   `name:"workers" help:"how many repositories are detected at the same time" default:"1" hidden:"true"`
}

// Run detects the workloads of the repositories. the workers of the detecting pool hand the ways each workload runs
// to the printing pool, which prints them as they come: in the order of the roots as given with one worker, otherwise
// as the repositories finish. a repository that fails is logged and does not hide the others, the first failure is
// the error.
func (t detectCmd) Run(kctx *kong.Context, gctx *cmdopts.Global, environ cmdopts.Environ) (err error) {
	ctx, cancel := context.WithCancel(gctx.Context)
	defer cancel()

	table := tabwriter.NewWriter(kctx.Stdout, 0, 4, 2, ' ', 0)
	if !t.JSONL {
		fmt.Fprintln(table, "WORKLOAD\tSTATUS\tCOMMAND")
	}

	// one worker, the output is written by one routine and in the order it was handed over.
	printing := asynccompute.New(func(_ context.Context, w *egcompute.Workload) (err error) {
		defer func() {
			// the output is gone, stop detecting.
			if err != nil {
				cancel()
			}
		}()

		if t.JSONL {
			return errorsx.Wrap(jsonx.Encode(kctx.Stdout, w), "unable to encode the workloads")
		}

		name := w.Workload
		if name == "" {
			name = "(default)"
		}

		status := "enabled"
		if w.Ignored {
			status = "disabled"
		}

		_, err = fmt.Fprintf(table, "%s\t%s\t%s\n", name, status, strings.Join(w.Command, " "))
		return err
	}, asynccompute.Compose(asynccompute.Workers[*egcompute.Workload](1), asynccompute.Backlog[*egcompute.Workload](128)))

	detecting := asynccompute.New(func(ctx context.Context, root string) error {
		// the canonical vcs uri, blank when the root is not a repository or has no origin remote.
		var uri string
		if repo, err := git.PlainOpen(root); err == nil {
			uri = errorsx.ZeroSilent(gitx.CanonicalURI(repo, git.DefaultRemoteName))
		}

		workloads := workspaces.DetectWorkloads(root, environ)
		for d := range workloads.Each(ctx) {
			if err := printing.Run(ctx, &egcompute.Workload{Workload: d.Workload, Directory: root, Command: d.Command, Vcsuri: uri, Ignored: d.Ignored}); err != nil {
				return err
			}
		}

		return errorsx.Wrapf(workloads.Err(), "unable to detect the workloads of %s", root)
	}, asynccompute.Compose(asynccompute.Workers[string](max(t.Workers, 1)), asynccompute.Backlog[string](uint16(len(t.Roots)+1))))

	// the backlog holds every root, feeding does not block.
	var feeding error
	for _, root := range t.Roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			feeding = errorsx.Wrapf(err, "invalid root %s", root)
			break
		}

		if feeding = detecting.Run(ctx, abs); feeding != nil {
			break
		}
	}

	// printing is shut down once detecting is done with it. both log the failures as they happen. what was found is
	// printed even when a repository failed. the parent context is waited on, ours ends when the output fails and
	// the workers stop promptly then.
	return errors.Join(feeding, asynccompute.Shutdown(gctx.Context, detecting, printing), table.Flush())
}
