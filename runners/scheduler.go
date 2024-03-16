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
		backoff.Maximum(3*time.Second),
	)

	spool := DefaultSpoolDirs()
	for {
		select {
		case <-ctx.Done():
			log.Println("auto enqueue done", ctx.Err())
			return
		case <-w.Await(s):
		}

		if dent, err := os.ReadDir(spool.Queued); err != nil {
			log.Println(errors.Wrap(err, "unable to read spool directory"))
			continue
		} else {
			log.Println("dent", len(dent))
		}

		if err := NewDownloadClient(authedclient).Download(ctx); err != nil {
			log.Println(errors.Wrap(err, "unable to download work"))
			continue
		}

		w.Reset() // reset attempts
	}
}
