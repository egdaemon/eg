package runners

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/pkg/errors"
)

func AutoDownload(ctx context.Context, authedclient *http.Client) {
	w := backoff.Waiter()
	s := backoff.New(
		backoff.Exponential(200*time.Millisecond),
		backoff.Maximum(30*time.Second),
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
			log.Println(errors.Wrap(err, "unable to read spool directory"))
			continue
		} else if len(dent) > 0 {
			log.Println("current tasks are in the running queue, not downloading any new tasks", len(dent))
			// fsx.PrintFS(os.DirFS(spool.Running))
			continue
		}

		if dent, err := os.ReadDir(spool.Queued); err != nil {
			log.Println(errors.Wrap(err, "unable to read spool directory"))
			continue
		} else if len(dent) > 0 {
			log.Println("current tasks are queued, not downloading any new tasks", len(dent))
			// fsx.PrintFS(os.DirFS(spool.Queued))
			continue
		}

		if err := NewDownloadClient(authedclient).Download(ctx); err != nil {
			log.Println(errors.Wrap(err, "unable to download work"))
			continue
		}

		w.Reset() // reset attempts
	}
}
