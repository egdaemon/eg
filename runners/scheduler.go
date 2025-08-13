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
	"github.com/egdaemon/eg/internal/numericx"
)

func workloadtarget() float64 {
	return envx.Float64(0.8, eg.EnvComputeWorkloadTargetLoad)
}

func AutoDownload(ctx context.Context, authedclient *http.Client, m *ResourceManager) {
	w := backoff.Chan()
	s := backoff.New(
		backoff.Exponential(200*time.Millisecond),
		backoff.Maximum(envx.Duration(time.Minute, eg.EnvScheduleMaximumDelay)),
		backoff.Jitter(0.02),
	)

	spool := DefaultSpoolDirs()

	determineload := func(limits, consumed RuntimeResources) float64 {
		cores := float64(consumed.Cores) / float64(limits.Cores)
		memory := float64(consumed.Memory) / float64(limits.Memory)
		log.Println("load", cores, memory, numericx.Max(cores, memory))
		return numericx.Max(cores, memory)
	}

	capacity := workloadcapacity()
	targetload := workloadtarget()
	targetlower := targetload / 2
	for {
		select {
		case <-ctx.Done():
			log.Println("auto enqueue done", ctx.Err())
			return
		case <-w.Await(s):
		case <-m.Completed():
		}

		if dent, err := os.ReadDir(spool.Running); err != nil {
			log.Println(errorsx.Wrap(err, "unable to read spool running directory"))
			continue
		} else if len(dent) >= capacity {
			log.Printf("current tasks are in the running queue, not downloading any new tasks r(%d) >= cap(%d)\n", len(dent), capacity)
			continue
		}

		if dent, err := os.ReadDir(spool.Queued); err != nil {
			log.Println(errorsx.Wrap(err, "unable to read spool queued directory"))
			continue
		} else if len(dent) > 0 {
			log.Println("current tasks are queued, not downloading any new tasks", len(dent))
			continue
		}

		var (
			err      error
			workload *EnqueuedDequeueResponse
			c        = m.Snapshot()
		)

		if workload, err = NewWorkloadClient(authedclient, m).Download(ctx, c); err != nil {
			log.Println(errorsx.Wrap(err, "unable to locate workload"))
			continue
		}

		wants := NewRuntimeResourcesFromDequeued(workload.Enqueued)
		if determineload(m.Limit, c.Reserve(wants)) > targetload {
			continue
		}

		if err = NewDownloadClient(authedclient).Download(ctx, workload); err != nil {
			log.Println(errorsx.Wrap(err, "unable to download workload"))
			continue
		}

		if currentload := determineload(m.Limit, c.Reserve(wants)); currentload > targetlower {
			continue
		}

		w.Reset() // reset attempts
	}
}
