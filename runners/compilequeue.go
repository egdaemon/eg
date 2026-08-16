package runners

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/backoff"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iox"

	"github.com/alitto/pond/v2"
)

func compilecapacity() int {
	return envx.Int(1, eg.EnvComputeCompileCapacity)
}

// Compile runs the compile worker pool, sized by EG_COMPUTE_COMPILE_CAPACITY,
// until ctx is cancelled.
func Compile(ctx context.Context, c *http.Client, compiledirs, rundirs SpoolDirs) error {
	return CompileN(ctx, compilecapacity(), c, compiledirs, rundirs)
}

// AutoCompile runs the compile worker pool in the background, logging
// (rather than returning) any terminal error -- mirrors AutoDownload's
// signature (scheduler.go) so callers can launch it directly via `go`.
func AutoCompile(ctx context.Context, c *http.Client, compiledirs, rundirs SpoolDirs) {
	if err := Compile(ctx, c, compiledirs, rundirs); err != nil {
		log.Println(errorsx.Wrap(err, "compile pool stopped"))
	}
}

// CompileN runs n workers, each claiming source-ref submissions from
// compiledirs and handing fully compiled workloads directly to
// rundirs.Queued (see compileWorkload). It runs until ctx is cancelled. n is
// the cap on concurrent compilations, independent of workloadcapacity (which
// caps concurrent running workloads only).
func CompileN(ctx context.Context, n int, c *http.Client, compiledirs, rundirs SpoolDirs) error {
	pool := pond.NewPool(n)
	workers := make([]pond.Task, 0, pool.MaxConcurrency())

	for i := 0; i < pool.MaxConcurrency(); i++ {
		workers = append(workers, pool.SubmitErr(func() error {
			return compileOne(ctx, c, compiledirs, rundirs)
		}))
	}

	pool.StopAndWait()

	for _, t := range workers {
		if err := t.Wait(); err != nil {
			return err
		}
	}

	return nil
}

// compileOne loops, claiming and compiling one job at a time, until ctx is
// cancelled. Compile jobs don't need the execution state machine or repo
// cache/lock resolution that beginwork/staterunning use -- a failed compile
// is simply discarded (terminal for that job), unlike a load-based rejection
// which is handled synchronously by the /c/enqueue handler before a job ever
// reaches this stage.
func compileOne(ctx context.Context, c *http.Client, compiledirs, rundirs SpoolDirs) error {
	s := backoff.New(
		backoff.Exponential(200*time.Millisecond),
		backoff.Maximum(envx.Duration(time.Minute, eg.EnvScheduleMaximumDelay)),
		backoff.Jitter(0.02),
	)
	w := backoff.Chan()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.Await(s):
		}

		dir, err := compiledirs.Dequeue()
		if err != nil {
			if iox.IgnoreEOF(err) != nil {
				log.Println("unable to dequeue compile job", err)
			}
			continue
		}

		log.Println("compiling workload initiated", dir)
		if err := compileWorkload(ctx, c, dir, rundirs); err != nil {
			log.Println(errorsx.Wrap(err, "compile failed"))
			errorsx.Log(errorsx.Wrap(compiledirs.Discard(dir), "failed to clear failed compile job"))
			continue
		}
		log.Println("compiling workload completed", dir)

		w.Reset()
	}
}
