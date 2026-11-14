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
	"github.com/egdaemon/eg/cmd/migrations"
	egcompute "github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/servesessions"
	"github.com/egdaemon/eg/jsonx"
)

type sessionsCmd struct {
	JSONL bool `name:"jsonl" help:"print the sessions as json lines, one session per line, for tools such as editor extensions"`
}

func (t sessionsCmd) Run(kctx *kong.Context, gctx *cmdopts.Global) (err error) {
	db, err := migrations.OpenLocked(gctx.Context, migrations.Path(migrations.CLI))
	if err != nil {
		return errorsx.Wrap(err, "unable to open the cli database")
	}
	defer db.Close()

	return writeSessions(gctx.Context, db, kctx.Stdout, t.JSONL)
}

// writeSessions prints the sessions served by 'eg compute serve --ssh', the running ones first.
func writeSessions(ctx context.Context, db *sql.DB, out io.Writer, asjsonl bool) (err error) {
	sessions, err := servesessions.List(ctx, db)
	if err != nil {
		return err
	}

	if asjsonl {
		for _, s := range sessions {
			encoded := &egcompute.ServeSession{
				Id: s.ID, Module: s.Module, Directory: s.Directory, Pid: s.PID, SshRequested: s.SSHRequested,
				SshPort: s.SSHPort, SshUser: s.SSHUser, SshKeyPath: s.SSHKeyPath,
				StartedAt: s.StartedAt.Format(time.RFC3339Nano),
				StoppedAt: stoppedAt(s.StoppedAt),
			}

			if err = jsonx.Encode(out, encoded); err != nil {
				return errorsx.Wrap(err, "unable to encode the sessions")
			}
		}

		return nil
	}

	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tMODULE\tDIRECTORY\tSTATE\tSSH PORT\tSTARTED")
	for _, s := range sessions {
		state, port := "stopped", "-"
		if s.Running() {
			state, port = "running", fmt.Sprint(s.SSHPort)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", s.ID, s.Module, s.Directory, state, port, s.StartedAt.Local().Format(time.DateTime))
	}

	return w.Flush()
}

func stoppedAt(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format(time.RFC3339Nano)
}
