package compute

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdtypography"
	"github.com/egdaemon/eg/cmd/migrations"
	egcompute "github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computesess"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/jsonx"
)

// sessionsCmd the sessions of 'eg compute serve' and 'eg compute local', listing them is the default.
type sessionsCmd struct {
	List   sessionsListCmd   `cmd:"" default:"withargs" help:"list the workloads served with 'eg compute serve --ssh', running and previous"`
	Rm     sessionsRmCmd     `cmd:"" help:"remove stopped sessions from the history"`
	Branch sessionsBranchCmd `cmd:"" help:"rename the branch of a session, running or stopped, for tools that manage the branch"`
}

type sessionsListCmd struct {
	JSONL bool `name:"jsonl" help:"print the sessions as json lines, one session per line, for tools such as editor extensions"`
}

func (t sessionsListCmd) Run(kctx *kong.Context, gctx *cmdopts.Global) (err error) {
	db, err := migrations.OpenLocked(gctx.Context, migrations.Path(migrations.CLI))
	if err != nil {
		return errorsx.Wrap(err, "unable to open the cli database")
	}
	defer db.Close()

	return writeSessions(gctx.Context, db, kctx.Stdout, t.JSONL)
}

type sessionsRmCmd struct {
	IDs []string `arg:"" name:"id" help:"ids of the sessions to remove, they must have stopped"`
}

func (t sessionsRmCmd) Run(kctx *kong.Context, gctx *cmdopts.Global) (err error) {
	db, err := migrations.OpenLocked(gctx.Context, migrations.Path(migrations.CLI))
	if err != nil {
		return errorsx.Wrap(err, "unable to open the cli database")
	}
	defer db.Close()

	return removeSessions(gctx.Context, db, kctx.Stdout, t.IDs)
}

type sessionsBranchCmd struct {
	ID     string `arg:"" name:"id" help:"id of the session"`
	Branch string `arg:"" name:"branch" help:"the new name of the branch"`
}

func (t sessionsBranchCmd) Run(gctx *cmdopts.Global) (err error) {
	db, err := migrations.OpenLocked(gctx.Context, migrations.Path(migrations.CLI))
	if err != nil {
		return errorsx.Wrap(err, "unable to open the cli database")
	}
	defer db.Close()

	return computesess.Rename(gctx.Context, db, t.ID, t.Branch)
}

// removeSessions forgets the stopped sessions with the given ids, printing the ones that were removed.
func removeSessions(ctx context.Context, db *sql.DB, out io.Writer, ids []string) (err error) {
	removed, err := computesess.Remove(ctx, db, ids...)
	for _, id := range removed {
		fmt.Fprintln(out, id)
	}

	return err
}

// writeSessions prints the sessions served by 'eg compute serve --ssh', the running ones first.
func writeSessions(ctx context.Context, db *sql.DB, out io.Writer, asjsonl bool) (err error) {
	sessions, err := computesess.List(ctx, db)
	if err != nil {
		return err
	}

	if asjsonl {
		for _, s := range sessions {
			encoded := &egcompute.ServeSession{
				Id: s.ID, Module: s.Module, Directory: s.Directory, Branch: s.Branch, Pid: s.Pid, SshRequested: s.SSHRequested,
				SshPort: s.SSHPort, SshUser: s.SSHUser, SshKeyPath: s.SSHKeyPath,
				CapSsh: s.CapSSH, CapAgent: s.CapAgent, CapPatch: s.CapPatch,
				StartedAt: fmt.Sprint(cmdtypography.Timestamp(s.StartedAt).Location(time.UTC)),
				StoppedAt: fmt.Sprint(cmdtypography.Timestamp(langx.DerefOrZero(s.StoppedAt)).Location(time.UTC).Zero("")), // blank while running, which the console reads as never.
			}

			if err = jsonx.Encode(out, encoded); err != nil {
				return errorsx.Wrap(err, "unable to encode the sessions")
			}
		}

		return nil
	}

	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tMODULE\tDIRECTORY\tBRANCH\tSTATE\tSSH PORT\tSTARTED")
	for _, s := range sessions {
		state, port := "stopped", "-"
		if s.Running() {
			state, port = "running", fmt.Sprint(s.SSHPort)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", s.ID, s.Module, s.Directory, s.Branch, state, port, s.StartedAt.Local().Format(time.DateTime))
	}

	return w.Flush()
}
