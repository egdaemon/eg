package compute

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/schedules"
)

// run enqueues one of the repository's scheduled workloads immediately, the way the scheduler would at its next
// slot and the console's run now button does. paused schedules can be run, pausing only stops the scheduler.
type run struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	ID          string `name:"id" help:"id of the schedule, selects one explicitly when the workload matches several schedules"`
	Follow      bool   `name:"follow" help:"stream the workload's logs once it is running, requires a runner supporting log streaming" negatable:"" default:"true"`
	Name        string `arg:"" name:"workload" help:"name of the scheduled workload to run, i.e. the workload of the schedule" default:""`
}

func (t run) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	if stringsx.Blank(t.Name) && stringsx.Blank(t.ID) {
		return errors.New("a workload or --id is required")
	}

	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	aid, err := cmdidentity.AccountID(ident.With(identity.QueryVCSAuto(gctx.AccountID, gctx.ProfileID, t.VcsURI)).Resolve(tlsc.DefaultClient(), signer))
	if err != nil {
		return err
	}

	return t.enqueue(gctx.Context, authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, gctx.ProfileID, compute.NewAuthzTokenSource))
}

// enqueue finds the schedule and enqueues it using the authorized client.
func (t run) enqueue(ctx context.Context, c *http.Client) (err error) {
	// a single page, the api has no exact filter on the workload and the server limits a page to 100.
	page, err := schedules.Search(ctx, c, &schedules.ScheduleSearchRequest{VcsUri: t.VcsURI, Limit: 100})
	if err != nil {
		return err
	}

	sched, err := matchSchedule(page.GetItems(), t.VcsURI, t.Name, t.ID)
	if err != nil {
		return err
	}

	recorded, err := computeapi.NewPushClient(c).Enqueue(ctx, computeapi.EnqueueFromSchedule(sched))
	if err != nil {
		return errorsx.Wrap(err, "unable to enqueue the workload")
	}

	log.Println("enqueued", recorded.GetId(), sched.GetEntry())

	if !t.Follow {
		log.Println("follow it with: eg compute watch", recorded.GetId())
		return nil
	}

	// enqueueing only records a job, the workload is queued once the cluster has prepared it.
	queued, err := awaitworkload(ctx, c, recorded.GetId(), awaitinterval, awaittimeout)
	if err != nil {
		return err
	}

	// a workload that already completed is gone from the queue, only its stored logs remain.
	if !queued {
		return computeapi.DownloadLogs(ctx, c, recorded.GetId(), os.Stdout)
	}

	return followenqueued(ctx, c, recorded.GetId())
}

const (
	// how often the cluster is polled for the enqueued workload to appear.
	awaitinterval = 2 * time.Second
	// how long the cluster has to prepare the workload and queue it before giving up, i.e.) cloning and compiling it.
	awaittimeout = 5 * time.Minute
)

// awaitworkload polls the cluster until the enqueued workload exists, which can take a while since enqueueing only
// records a job. queued reports whether it is in the queue, waiting for or running on a runner, otherwise it already
// completed and only exists in the history.
func awaitworkload(parent context.Context, c *http.Client, id string, interval, timeout time.Duration) (queued bool, err error) {
	ctx, done := context.WithTimeout(parent, timeout)
	defer done()

	log.Println("waiting for the workload to be queued", id)

	tick := time.NewTicker(interval)
	defer tick.Stop()

	client := computeapi.NewPushClient(c)

	// the deadline can expire within a request, which fails with the context's error rather than the poll's.
	expired := func(cause error) error {
		if parent.Err() != nil {
			return cause
		}

		return errorsx.Wrapf(context.DeadlineExceeded, "the workload %s was never queued, it may have failed to prepare, see eg compute history", id)
	}

	for {
		_, err = client.Find(ctx, id)
		if err == nil {
			return true, nil
		} else if ctx.Err() != nil {
			return false, expired(err)
		} else if httpx.IsStatusError(err, http.StatusNotFound) == nil {
			return false, errorsx.Wrap(err, "unable to locate the workload on the cluster")
		}

		_, err = client.HistoryFind(ctx, id)
		if err == nil {
			return false, nil
		} else if ctx.Err() != nil {
			return false, expired(err)
		} else if httpx.IsStatusError(err, http.StatusNotFound) == nil {
			return false, errorsx.Wrap(err, "unable to locate the workload in the history")
		}

		select {
		case <-tick.C:
		case <-ctx.Done():
			return false, expired(ctx.Err())
		}
	}
}

// matchSchedule the single schedule whose workload equals the one requested and, when provided, whose id equals id.
// zero matches and multiple matches are errors, the latter lists the candidates so one can be selected by id.
func matchSchedule(items []*schedules.Schedule, vcsuri, workload, id string) (*schedules.Schedule, error) {
	var found []*schedules.Schedule

	for _, s := range items {
		if stringsx.Present(workload) && s.GetEntry() != workload {
			continue
		}

		if stringsx.Present(id) && s.GetId() != id {
			continue
		}

		found = append(found, s)
	}

	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return nil, fmt.Errorf("no scheduled workload matched in %s: workload %q id %q", stringsx.DefaultIfBlank(vcsuri, "all repositories"), workload, id)
	}

	var b strings.Builder
	for _, s := range found {
		state := "active"
		if schedules.Paused(s) {
			state = "paused"
		}
		fmt.Fprintf(&b, "\n  id %s workload %s schedule %s %s %q", s.GetId(), s.GetEntry(), s.GetCron(), state, s.GetDescription())
	}

	return nil, fmt.Errorf("ambiguous, %d scheduled workloads matched %q, select one with --id:%s", len(found), workload, b.String())
}
