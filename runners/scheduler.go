package runners

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/backoff"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/tracex"
)

func AutoDownload(ctx context.Context, authedclient *http.Client) {
	w := backoff.Chan()
	s := backoff.New(
		backoff.Exponential(200*time.Millisecond),
		backoff.Maximum(envx.Duration(time.Minute, eg.EnvScheduleMaximumDelay)),
		backoff.Jitter(0.02),
	)

	auto := NewRuntimeResources()
	spool := DefaultSpoolDirs()

	for {
		select {
		case <-ctx.Done():
			log.Println("auto enqueue done", ctx.Err())
			return
		case <-w.Await(s):
		}

		if dent, err := os.ReadDir(spool.Running); err != nil {
			log.Println(errorsx.Wrap(err, "unable to read spool running directory"))
			continue
		} else if len(dent) > 0 {
			tracex.Println("current tasks are in the running queue, not downloading any new tasks", len(dent))
			continue
		}

		if dent, err := os.ReadDir(spool.Queued); err != nil {
			log.Println(errorsx.Wrap(err, "unable to read spool queued directory"))
			continue
		} else if len(dent) > 0 {
			tracex.Println("current tasks are queued, not downloading any new tasks", len(dent))
			continue
		}

		if err := NewDownloadClient(authedclient).Download(ctx, auto); err != nil {
			log.Println(errorsx.Wrap(err, "unable to download work"))
			continue
		}

		w.Reset() // reset attempts
	}
}
