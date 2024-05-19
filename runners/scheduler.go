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
)

func AutoDownload(ctx context.Context, authedclient *http.Client) {
	w := backoff.Waiter()
	s := backoff.New(
		backoff.Exponential(envx.Duration(200*time.Millisecond, eg.EnvScheduleMaximumDelay)),
		backoff.Maximum(1*time.Minute),
		backoff.Jitter(0.02),
	)

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
			log.Println("current tasks are in the running queue, not downloading any new tasks", len(dent))
			continue
		}

		if dent, err := os.ReadDir(spool.Queued); err != nil {
			log.Println(errorsx.Wrap(err, "unable to read spool queued directory"))
			continue
		} else if len(dent) > 0 {
			log.Println("current tasks are queued, not downloading any new tasks", len(dent))
			continue
		}

		if err := NewDownloadClient(authedclient).Download(ctx); err != nil {
			log.Println(errorsx.Wrap(err, "unable to download work"))
			continue
		}

		w.Reset() // reset attempts
	}
}
