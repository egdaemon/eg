package computex

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

// ErrNotRecorded the completion wasn't recorded in time.
const ErrNotRecorded = errorsx.String("result was not recorded")

// Unavailable marks a result that could not be retrieved, as opposed to a workload that did not succeed.
type Unavailable struct {
	Cause error
}

func (t Unavailable) Error() string {
	return "result unavailable: " + t.Cause.Error()
}

func (t Unavailable) Unwrap() error {
	return t.Cause
}

// Finder retrieves the history record of a workload, a not found status means the completion isn't recorded yet.
type Finder func(ctx context.Context, id string) (*compute.History, error)

// Await reports how the completed workload ended, nil when it succeeded. the finders are tried in order, falling
// back to the next when one cannot answer: failing to retrieve the record (Unavailable) or a record that never
// appears (ErrNotRecorded). the error of the last finder is returned when none can.
func Await(ctx context.Context, id string, finds ...Finder) (err error) {
	if len(finds) == 0 {
		return errorsx.String("no history sources provided")
	}

	for i, find := range finds {
		if err = await(ctx, id, find, awaitattempts, awaitdelay); !errors.As(err, &Unavailable{}) && !errors.Is(err, ErrNotRecorded) {
			return err
		}

		if i < len(finds)-1 {
			log.Println("history unavailable, falling back to the next source", id, err)
		}
	}

	return err
}

// AwaitFor is Await for a workload that may still be running, i.e.) its logs could not be streamed. rather than
// the short grace Await allows for a record trailing the end of a log stream, each finder is polled until the
// workload reaches a terminal state or the duration elapses, in which case the result is ErrNotRecorded.
func AwaitFor(ctx context.Context, id string, d time.Duration, finds ...Finder) (err error) {
	if len(finds) == 0 {
		return errorsx.String("no history sources provided")
	}

	tctx, done := context.WithTimeout(ctx, d)
	defer done()

	for i, find := range finds {
		if err = await(tctx, id, find, 0, min(awaitlongdelay, d/20)); !errors.As(err, &Unavailable{}) && !errors.Is(err, ErrNotRecorded) {
			break
		}

		if i < len(finds)-1 {
			log.Println("history unavailable, falling back to the next source", id, err)
		}
	}

	// only the deadline ended, the caller is still going.
	if ctx.Err() == nil && tctx.Err() != nil && errors.Is(err, context.DeadlineExceeded) {
		return errorsx.Wrapf(ErrNotRecorded, "workload %s", id)
	}

	return err
}

const (
	// the history record is written by the completion and can trail the end of the log stream.
	awaitattempts = 10
	awaitdelay    = time.Second
	// how often a workload that may still be running is polled.
	awaitlongdelay = 5 * time.Second
)

// await polls a single finder, attempts of zero polls until the context ends. failing to retrieve the record
// is Unavailable, and a record that never appears is ErrNotRecorded.
func await(ctx context.Context, id string, find Finder, attempts int, delay time.Duration) (err error) {
	for i := 0; attempts == 0 || i < attempts; i++ {
		var h *compute.History

		// not found and unknown results mean the completion hasn't been recorded yet.
		if h, err = find(ctx, id); err == nil {
			switch h.Result {
			case ResultCompleted:
				return nil
			case ResultFailure:
				return errorsx.Errorf("workload %s failed", id)
			case ResultCancelled:
				return errorsx.Errorf("workload %s was cancelled", id)
			}
		} else if httpx.IsStatusError(err, http.StatusNotFound) == nil {
			return Unavailable{Cause: err}
		}

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return errorsx.Wrapf(ErrNotRecorded, "workload %s", id)
}

// Logs writes the stored log of a workload that completed before its logs could be followed into dst.
// fetch retrieves the log, a not found status means it isn't recorded yet.
func Logs(ctx context.Context, id string, dst io.Writer, fetch func(ctx context.Context, id string, dst io.Writer) error) (err error) {
	// the history record is written by the completion and can trail the workload's completion.
	const (
		attempts = 5
		delay    = time.Second
	)

	for range attempts {
		if err = fetch(ctx, id, dst); httpx.IsStatusError(err, http.StatusNotFound) == nil {
			return errorsx.Wrap(err, "stored workload logs unavailable")
		}

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return errorsx.Wrap(err, "stored workload logs unavailable")
}
