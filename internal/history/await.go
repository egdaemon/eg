package history

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
		if err = await(ctx, id, find); !errors.As(err, &Unavailable{}) && !errors.Is(err, ErrNotRecorded) {
			return err
		}

		if i < len(finds)-1 {
			log.Println("history unavailable, falling back to the next source", id, err)
		}
	}

	return err
}

// await polls a single finder. failing to retrieve the record is Unavailable, and a record that never appears
// is ErrNotRecorded.
func await(ctx context.Context, id string, find Finder) (err error) {
	// the history record is written by the completion and can trail the end of the log stream.
	const (
		attempts = 10
		delay    = time.Second
	)

	for range attempts {
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
